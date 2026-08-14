//go:build windows

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	createRestrictedWriteRestricted  = 0x8
	createRestrictedDisableMaxPriv   = 0x1
	seGroupLogonID                   = 0xC0000000
	winACLGrantMask windows.ACCESS_MASK = windows.FILE_GENERIC_READ |
		windows.FILE_GENERIC_WRITE |
		windows.FILE_GENERIC_EXECUTE |
		windows.DELETE |
		0x40 // FILE_DELETE_CHILD
)

var (
	modAdvapi32             = windows.NewLazySystemDLL("advapi32.dll")
	procCreateRestrictedTok = modAdvapi32.NewProc("CreateRestrictedToken")
)

type sidAndAttributes struct {
	Sid        *windows.SID
	Attributes uint32
}

// WinACL confines children with WRITE_RESTRICTED tokens and directory ACEs.
// Enforcement is always partial (Everyone keep-alive + NTFS hard-link gaps).
type WinACL struct {
	mu       sync.Mutex
	probed   bool
	probeOK  bool
	probeErr error
}

func (w *WinACL) Name() string { return "winacl" }

func (w *WinACL) Probe(ctx context.Context) (bool, Enforcement, error) {
	_ = ctx
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.probed {
		return w.probeOK, EnforcementPartial, w.probeErr
	}
	w.probed = true
	token, err := openCurrentToken()
	if err != nil {
		w.probeOK = false
		w.probeErr = &UnavailableError{Backend: w.Name(), Reason: err.Error()}
		return false, "", w.probeErr
	}
	defer token.Close()
	logon, err := logonSID(token)
	if err != nil {
		w.probeOK = false
		w.probeErr = &UnavailableError{Backend: w.Name(), Reason: "logon SID: " + err.Error()}
		return false, "", w.probeErr
	}
	everyone, err := windows.StringToSid("S-1-1-0")
	if err != nil {
		w.probeOK = false
		w.probeErr = &UnavailableError{Backend: w.Name(), Reason: err.Error()}
		return false, "", w.probeErr
	}
	restricted, err := createRestrictedToken(token, []sidAndAttributes{
		{Sid: logon},
		{Sid: everyone},
	})
	if err != nil {
		w.probeOK = false
		w.probeErr = &UnavailableError{Backend: w.Name(), Reason: "CreateRestrictedToken: " + err.Error()}
		return false, "", w.probeErr
	}
	_ = windows.CloseHandle(windows.Handle(restricted))
	w.probeOK = true
	return true, EnforcementPartial, nil
}

func (w *WinACL) Confine(ctx context.Context, program string, args []string, policy Policy) (Confined, error) {
	_ = ctx
	_ = program
	_ = args
	_ = policy
	return Confined{}, &UnavailableError{
		Backend: w.Name(),
		Reason:  "winacl requires PrepareCommand (restricted tokens cannot be argv-wrapped)",
	}
}

func (w *WinACL) PrepareCommand(ctx context.Context, program string, args []string, dir string, env []string, policy Policy) (*exec.Cmd, Confined, error) {
	if err := ValidatePolicy(policy); err != nil {
		return nil, Confined{}, err
	}
	if !NeedsConfine(policy) {
		return nil, Confined{}, errors.New("sandbox: PrepareCommand called for non-confining mode")
	}
	ok, enforcement, err := w.Probe(ctx)
	if err != nil {
		return nil, Confined{}, err
	}
	if !ok {
		return nil, Confined{}, &UnavailableError{Backend: w.Name(), Reason: "probe reported unavailable"}
	}

	root, err := canonicalWindowsPath(policy.WorkspaceRoot)
	if err != nil {
		return nil, Confined{}, err
	}
	tempDir, err := ensureSessionTempGeneric(policy.SessionID, root)
	if err != nil {
		return nil, Confined{}, err
	}
	tempDir, err = canonicalWindowsPath(tempDir)
	if err != nil {
		return nil, Confined{}, err
	}
	if pathsOverlap(root, tempDir) {
		return nil, Confined{}, &UnavailableError{Backend: w.Name(), Reason: "workspace and temp paths overlap"}
	}

	var releaseFuncs []func()
	release := func() {
		for i := len(releaseFuncs) - 1; i >= 0; i-- {
			releaseFuncs[i]()
		}
	}
	fail := func(err error) (*exec.Cmd, Confined, error) {
		release()
		return nil, Confined{}, err
	}

	source, err := openCurrentToken()
	if err != nil {
		return fail(err)
	}
	releaseFuncs = append(releaseFuncs, func() { _ = source.Close() })

	logon, err := logonSID(source)
	if err != nil {
		return fail(fmt.Errorf("logon SID: %w", err))
	}
	everyone, err := windows.StringToSid("S-1-1-0")
	if err != nil {
		return fail(err)
	}

	restricting := []sidAndAttributes{{Sid: logon}, {Sid: everyone}}
	switch policy.Mode {
	case ModeReadOnly:
		// No write SIDs: standing workspace ACEs stay inert under WRITE_RESTRICTED.
	case ModeWorkspaceWrite:
		wsSID, err := windows.StringToSid(WorkspaceWriteSIDString(root))
		if err != nil {
			return fail(err)
		}
		tmpSID, err := windows.StringToSid(TempWriteSIDString(tempDir))
		if err != nil {
			return fail(err)
		}
		if err := grantWriteACE(root, wsSID); err != nil {
			return fail(fmt.Errorf("workspace ACE: %w", err))
		}
		if err := grantWriteACE(tempDir, tmpSID); err != nil {
			return fail(fmt.Errorf("temp ACE: %w", err))
		}
		// Temp ACE is revocable; workspace ACE is standing (reuse cache).
		releaseFuncs = append(releaseFuncs, func() { _ = revokeWriteACE(tempDir, tmpSID) })
		restricting = append(restricting, sidAndAttributes{Sid: wsSID}, sidAndAttributes{Sid: tmpSID})
	default:
		return fail(fmt.Errorf("sandbox: unsupported mode %q", policy.Mode))
	}

	restricted, err := createRestrictedToken(source, restricting)
	if err != nil {
		return fail(fmt.Errorf("CreateRestrictedToken: %w", err))
	}
	releaseFuncs = append(releaseFuncs, func() { _ = windows.CloseHandle(windows.Handle(restricted)) })

	resolvedProgram, err := exec.LookPath(program)
	if err != nil {
		// Keep relative/absolute program as provided when LookPath fails.
		resolvedProgram = program
	}

	cmd := exec.CommandContext(ctx, resolvedProgram, args...)
	cmd.Dir = dir
	if len(env) == 0 {
		env = os.Environ()
	}
	cmd.Env = mergeEnvTMP(env, tempDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token: syscall.Token(restricted),
	}

	confined := Confined{
		Program:     resolvedProgram,
		Args:        append([]string(nil), args...),
		Env:         map[string]string{"TMP": tempDir, "TEMP": tempDir, "TMPDIR": tempDir},
		Enforcement: enforcement,
		DenialSignatures: []string{
			"access is denied",
			"access denied",
			"permission denied",
		},
		RunnerFailRules: []RunnerFailureRule{{
			FatalSignatures: []string{"windows-acl-run:", "CreateRestrictedToken", "winacl:"},
		}},
		Backend: w.Name(),
		Release: release,
	}
	return cmd, confined, nil
}

func openCurrentToken() (windows.Token, error) {
	var token windows.Token
	err := windows.OpenProcessToken(
		windows.CurrentProcess(),
		windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ADJUST_DEFAULT|windows.TOKEN_ASSIGN_PRIMARY,
		&token,
	)
	return token, err
}

func logonSID(token windows.Token) (*windows.SID, error) {
	groups, err := token.GetTokenGroups()
	if err != nil {
		return nil, err
	}
	for _, group := range groups.AllGroups() {
		if group.Attributes&seGroupLogonID != 0 && group.Sid != nil {
			return group.Sid.Copy()
		}
	}
	return nil, errors.New("token has no logon SID")
}

func createRestrictedToken(existing windows.Token, restricting []sidAndAttributes) (windows.Token, error) {
	if err := procCreateRestrictedTok.Find(); err != nil {
		return 0, err
	}
	var first uintptr
	if len(restricting) > 0 {
		first = uintptr(unsafe.Pointer(&restricting[0]))
	}
	var out windows.Token
	r1, _, e1 := procCreateRestrictedTok.Call(
		uintptr(existing),
		uintptr(createRestrictedWriteRestricted|createRestrictedDisableMaxPriv),
		0, 0,
		0, 0,
		uintptr(len(restricting)),
		first,
		uintptr(unsafe.Pointer(&out)),
	)
	if r1 == 0 {
		if e1 != nil && e1 != syscall.Errno(0) {
			return 0, e1
		}
		return 0, errors.New("CreateRestrictedToken failed")
	}
	return out, nil
}

func grantWriteACE(path string, sid *windows.SID) error {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	entry := windows.EXPLICIT_ACCESS{
		AccessPermissions: winACLGrantMask,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{entry}, dacl)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil,
	)
}

func revokeWriteACE(path string, sid *windows.SID) error {
	// Best-effort: grant DENY is wrong; remove by rebuilding without the SID is
	// complex. Leaving the ACE on a private temp directory is acceptable — the
	// directory is session-scoped under klaude-sbx and the restricting SID is
	// unique. Explicit revoke uses SetEntriesInAcl REVOKE_ACCESS.
	entry := windows.EXPLICIT_ACCESS{
		AccessPermissions: winACLGrantMask,
		AccessMode:        windows.REVOKE_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{entry}, dacl)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}

func canonicalWindowsPath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return "", err
		}
		cleaned = abs
	}
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return cleaned, nil
	}
	return resolved, nil
}

func pathsOverlap(a, b string) bool {
	a = strings.ToLower(filepath.Clean(a))
	b = strings.ToLower(filepath.Clean(b))
	sep := string(os.PathSeparator)
	if a == b {
		return true
	}
	if strings.HasPrefix(a, b+sep) || strings.HasPrefix(b, a+sep) {
		return true
	}
	return false
}

func mergeEnvTMP(base []string, tempDir string) []string {
	overrides := map[string]string{
		"TMP":    tempDir,
		"TEMP":   tempDir,
		"TMPDIR": tempDir,
	}
	index := map[string]int{}
	out := append([]string(nil), base...)
	for i, entry := range out {
		if key, _, ok := strings.Cut(entry, "="); ok {
			index[strings.ToUpper(key)] = i
		}
	}
	for key, value := range overrides {
		entry := key + "=" + value
		if i, ok := index[strings.ToUpper(key)]; ok {
			out[i] = entry
		} else {
			out = append(out, entry)
		}
	}
	return out
}
