//go:build linux

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Landlock wraps the landlock-run launcher when present on PATH (dsh-compatible).
// Older ABIs may report partial enforcement via launcher notices.
type Landlock struct {
	ExecPath string

	mu         sync.Mutex
	probed     bool
	probeOK    bool
	probeErr   error
	bin        string
	enforcement Enforcement
}

func (l *Landlock) Name() string { return "landlock" }

func (l *Landlock) binary() string {
	if l != nil && strings.TrimSpace(l.ExecPath) != "" {
		return l.ExecPath
	}
	return "landlock-run"
}

func (l *Landlock) Probe(ctx context.Context) (bool, Enforcement, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.probed {
		return l.probeOK, l.enforcement, l.probeErr
	}
	l.probed = true
	l.enforcement = EnforcementFull
	path, err := exec.LookPath(l.binary())
	if err != nil {
		l.probeOK = false
		l.probeErr = &UnavailableError{Backend: l.Name(), Reason: err.Error()}
		return false, "", l.probeErr
	}
	l.bin = path
	cmd := exec.CommandContext(ctx, path, "--probe")
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		l.probeOK = false
		l.probeErr = &UnavailableError{Backend: l.Name(), Reason: fmt.Sprintf("probe failed: %v (%s)", err, text)}
		return false, "", l.probeErr
	}
	if strings.Contains(strings.ToLower(text), "partial enforcement") {
		l.enforcement = EnforcementPartial
	}
	l.probeOK = true
	return true, l.enforcement, nil
}

func (l *Landlock) Confine(ctx context.Context, program string, args []string, policy Policy) (Confined, error) {
	if err := ValidatePolicy(policy); err != nil {
		return Confined{}, err
	}
	if !NeedsConfine(policy) {
		return Confined{}, errors.New("sandbox: confine called for non-confining mode")
	}
	ok, enforcement, err := l.Probe(ctx)
	if err != nil {
		return Confined{}, err
	}
	if !ok {
		return Confined{}, &UnavailableError{Backend: l.Name(), Reason: "probe reported unavailable"}
	}
	root, err := canonicalPath(policy.WorkspaceRoot)
	if err != nil {
		return Confined{}, err
	}
	tempDir, err := ensureLinuxSessionTemp(policy.SessionID, root)
	if err != nil {
		return Confined{}, err
	}
	argv := []string{}
	switch policy.Mode {
	case ModeReadOnly:
		argv = append(argv, "--ro", "/dev/null")
	case ModeWorkspaceWrite:
		argv = append(argv, "--rw", root, "--rw", tempDir, "--ro", "/dev/null")
	default:
		return Confined{}, fmt.Errorf("sandbox: unsupported mode %q", policy.Mode)
	}
	argv = append(argv, "--", program)
	argv = append(argv, args...)
	return Confined{
		Program:          l.bin,
		Args:             argv,
		Env:              map[string]string{"TMPDIR": tempDir, "TMP": tempDir, "TEMP": tempDir},
		Enforcement:      enforcement,
		DenialSignatures: []string{"operation not permitted", "permission denied", "eacces"},
		RunnerFailRules: []RunnerFailureRule{{
			AllowedExitCodes:   []int{125},
			FatalSignatures:    []string{"landlock-run:"},
			InformationalLines: []string{"landlock-run: partial enforcement (older Landlock ABI)"},
		}},
		Backend: l.Name(),
	}, nil
}
