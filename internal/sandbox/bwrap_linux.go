//go:build linux

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

// Bwrap confines children with bubblewrap bind mounts.
type Bwrap struct {
	ExecPath string

	mu       sync.Mutex
	probed   bool
	probeOK  bool
	probeErr error
	bin      string
}

func (b *Bwrap) Name() string { return "bwrap" }

func (b *Bwrap) binary() string {
	if b != nil && strings.TrimSpace(b.ExecPath) != "" {
		return b.ExecPath
	}
	return "bwrap"
}

func (b *Bwrap) Probe(ctx context.Context) (bool, Enforcement, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.probed {
		return b.probeOK, EnforcementFull, b.probeErr
	}
	b.probed = true
	path, err := exec.LookPath(b.binary())
	if err != nil {
		b.probeOK = false
		b.probeErr = &UnavailableError{Backend: b.Name(), Reason: err.Error()}
		return false, "", b.probeErr
	}
	b.bin = path
	cmd := exec.CommandContext(ctx, path, "--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc", "--die-with-parent", "true")
	if output, err := cmd.CombinedOutput(); err != nil {
		b.probeOK = false
		b.probeErr = &UnavailableError{
			Backend: b.Name(),
			Reason:  fmt.Sprintf("probe failed: %v (%s)", err, strings.TrimSpace(string(output))),
		}
		return false, "", b.probeErr
	}
	b.probeOK = true
	return true, EnforcementFull, nil
}

func (b *Bwrap) Confine(ctx context.Context, program string, args []string, policy Policy) (Confined, error) {
	if err := ValidatePolicy(policy); err != nil {
		return Confined{}, err
	}
	if !NeedsConfine(policy) {
		return Confined{}, errors.New("sandbox: confine called for non-confining mode")
	}
	ok, enforcement, err := b.Probe(ctx)
	if err != nil {
		return Confined{}, err
	}
	if !ok {
		return Confined{}, &UnavailableError{Backend: b.Name(), Reason: "probe reported unavailable"}
	}
	root, err := canonicalPath(policy.WorkspaceRoot)
	if err != nil {
		return Confined{}, err
	}
	tempDir, err := ensureLinuxSessionTemp(policy.SessionID, root)
	if err != nil {
		return Confined{}, err
	}
	argv := []string{
		"--ro-bind", "/", "/",
		"--dev", "/dev",
		"--proc", "/proc",
		"--die-with-parent",
	}
	switch policy.Mode {
	case ModeReadOnly:
		argv = append(argv, "--ro-bind", root, root)
	case ModeWorkspaceWrite:
		argv = append(argv,
			"--bind", root, root,
			"--bind", tempDir, tempDir,
		)
		for _, tmp := range []string{"/tmp", os.TempDir()} {
			resolved, err := canonicalPath(tmp)
			if err != nil || resolved == "" || resolved == root || resolved == tempDir {
				continue
			}
			argv = append(argv, "--bind", resolved, resolved)
		}
	default:
		return Confined{}, fmt.Errorf("sandbox: unsupported mode %q", policy.Mode)
	}
	argv = append(argv, "--chdir", root, "--", program)
	argv = append(argv, args...)
	return Confined{
		Program:     b.bin,
		Args:        argv,
		Env:         map[string]string{"TMPDIR": tempDir, "TMP": tempDir, "TEMP": tempDir},
		Enforcement: enforcement,
		DenialSignatures: []string{
			"read-only file system",
			"operation not permitted",
			"permission denied",
		},
		RunnerFailRules: []RunnerFailureRule{{
			FatalSignatures: []string{"bwrap:", "bubblewrap:"},
		}},
		Backend: b.Name(),
	}, nil
}

func canonicalPath(root string) (string, error) {
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

func ensureLinuxSessionTemp(sessionID, workspace string) (string, error) {
	return ensureSessionTempGeneric(sessionID, workspace)
}
