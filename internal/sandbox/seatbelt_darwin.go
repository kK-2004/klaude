//go:build darwin

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
)

const seatbeltBinary = "sandbox-exec"

// Seatbelt confines children with macOS sandbox-exec profiles.
// Profile style matches common agent harnesses: allow-default, deny file-write*,
// then allow-list mode-specific writable roots (enforcement: full when probe OK).
type Seatbelt struct {
	ExecPath string // optional override for tests

	mu       sync.Mutex
	probed   bool
	probeOK  bool
	probeErr error
}

func (s *Seatbelt) Name() string { return "seatbelt" }

func (s *Seatbelt) binary() string {
	if s != nil && strings.TrimSpace(s.ExecPath) != "" {
		return s.ExecPath
	}
	return seatbeltBinary
}

func (s *Seatbelt) Probe(ctx context.Context) (bool, Enforcement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.probed {
		return s.probeOK, EnforcementFull, s.probeErr
	}
	s.probed = true
	profile := readOnlyProfile("/dev/null")
	cmd := exec.CommandContext(ctx, s.binary(), "-p", profile, "--", "/usr/bin/true")
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.probeOK = false
		s.probeErr = &UnavailableError{
			Backend: s.Name(),
			Reason:  fmt.Sprintf("%s probe failed: %v (%s)", s.binary(), err, strings.TrimSpace(string(output))),
		}
		return false, "", s.probeErr
	}
	s.probeOK = true
	return true, EnforcementFull, nil
}

func (s *Seatbelt) Confine(ctx context.Context, program string, args []string, policy Policy) (Confined, error) {
	if s == nil {
		return Confined{}, &UnavailableError{Backend: "seatbelt", Reason: "nil backend"}
	}
	if err := ValidatePolicy(policy); err != nil {
		return Confined{}, err
	}
	if !NeedsConfine(policy) {
		return Confined{}, errors.New("sandbox: confine called for non-confining mode")
	}
	ok, enforcement, err := s.Probe(ctx)
	if err != nil {
		return Confined{}, err
	}
	if !ok {
		return Confined{}, &UnavailableError{Backend: s.Name(), Reason: "probe reported unavailable"}
	}

	root, err := canonicalRoot(policy.WorkspaceRoot)
	if err != nil {
		return Confined{}, err
	}
	tempDir, err := ensureSessionTempGeneric(policy.SessionID, root)
	if err != nil {
		return Confined{}, err
	}

	var profile string
	switch policy.Mode {
	case ModeReadOnly:
		profile = readOnlyProfile("/dev/null")
	case ModeWorkspaceWrite:
		writable := []string{root, tempDir}
		if tmp := darwinTempRoots(); len(tmp) > 0 {
			writable = append(writable, tmp...)
		}
		profile = workspaceWriteProfile("/dev/null", writable)
	default:
		return Confined{}, fmt.Errorf("sandbox: unsupported mode %q", policy.Mode)
	}

	bin := s.binary()
	path, lookErr := exec.LookPath(bin)
	if lookErr != nil {
		return Confined{}, &UnavailableError{Backend: s.Name(), Reason: lookErr.Error()}
	}

	confinedArgs := append([]string{"-p", profile, "--", program}, args...)
	env := map[string]string{
		"TMPDIR": tempDir,
		"TMP":    tempDir,
		"TEMP":   tempDir,
	}
	return Confined{
		Program:          path,
		Args:             confinedArgs,
		Env:              env,
		Enforcement:      enforcement,
		DenialSignatures: []string{"operation not permitted", "sandbox: deny", "eperm"},
		RunnerFailRules: []RunnerFailureRule{{
			FatalSignatures: []string{"sandbox-exec:", "failed to load profile", "unable to apply sandbox"},
		}},
		Backend: s.Name(),
	}, nil
}

func canonicalRoot(root string) (string, error) {
	cleaned := filepath.Clean(root)
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

func darwinTempRoots() []string {
	var roots []string
	for _, candidate := range []string{"/private/tmp", "/tmp", os.TempDir()} {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			resolved = filepath.Clean(candidate)
		}
		if resolved == "" {
			continue
		}
		dup := false
		for _, existing := range roots {
			if existing == resolved {
				dup = true
				break
			}
		}
		if !dup {
			roots = append(roots, resolved)
		}
	}
	return roots
}

func readOnlyProfile(nullPath string) string {
	var b strings.Builder
	b.WriteString("(version 1)\n")
	b.WriteString("(allow default)\n")
	b.WriteString("(deny file-write*)\n")
	b.WriteString("(allow file-write* (literal ")
	b.WriteString(sbString(nullPath))
	b.WriteString("))\n")
	return b.String()
}

func workspaceWriteProfile(nullPath string, writable []string) string {
	var b strings.Builder
	b.WriteString(readOnlyProfile(nullPath))
	for _, root := range writable {
		if strings.TrimSpace(root) == "" {
			continue
		}
		b.WriteString("(allow file-write* (subpath ")
		b.WriteString(sbString(root))
		b.WriteString("))\n")
	}
	return b.String()
}

func sbString(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}
