package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Mode is the file-effect policy for one confined spawn.
type Mode string

const (
	ModeReadOnly         Mode = "read_only"
	ModeWorkspaceWrite   Mode = "workspace_write"
	ModeDangerFullAccess Mode = "danger_full_access"
)

// Enforcement reports how completely the selected backend can keep the promise.
type Enforcement string

const (
	EnforcementFull    Enforcement = "full"
	EnforcementPartial Enforcement = "partial"
)

// Policy is resolved per call and passed to Confine.
type Policy struct {
	Mode          Mode
	WorkspaceRoot string
	SessionID     string
}

// RunnerFailureRule identifies sandbox infrastructure failure before the command runs.
type RunnerFailureRule struct {
	FatalSignatures     []string
	InformationalLines  []string
	AllowedExitCodes    []int // empty = any nonzero
}

// Confined is the argv/env the Executor must spawn instead of the caller's own.
type Confined struct {
	Program          string
	Args             []string
	Env              map[string]string
	Enforcement      Enforcement
	DenialSignatures []string
	RunnerFailRules  []RunnerFailureRule
	Backend          string
	// Release frees backend resources (e.g. Windows restricted token handles).
	Release func()
}

// Backend probes and wraps argv for one platform runner.
type Backend interface {
	Name() string
	Probe(ctx context.Context) (ok bool, enforcement Enforcement, err error)
	Confine(ctx context.Context, program string, args []string, policy Policy) (Confined, error)
}



// UnavailableError means confinement was required but no usable backend exists.
type UnavailableError struct {
	Backend string
	Reason  string
}

func (e *UnavailableError) Error() string {
	if e == nil {
		return "sandbox unavailable"
	}
	if e.Backend == "" {
		return fmt.Sprintf("sandbox unavailable: %s", e.Reason)
	}
	return fmt.Sprintf("sandbox %s unavailable: %s", e.Backend, e.Reason)
}

func (e *UnavailableError) Is(target error) bool {
	_, ok := target.(*UnavailableError)
	return ok
}

// ErrUnavailable is for errors.Is matching.
var ErrUnavailable = &UnavailableError{Reason: "no usable backend"}

// ParseMode accepts config / API spellings.
func ParseMode(value string) (Mode, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", string(ModeWorkspaceWrite), "workspace-write":
		return ModeWorkspaceWrite, nil
	case string(ModeReadOnly), "read-only", "readonly":
		return ModeReadOnly, nil
	case string(ModeDangerFullAccess), "danger-full-access", "full_access", "full-access":
		return ModeDangerFullAccess, nil
	default:
		return "", fmt.Errorf("sandbox: unknown mode %q", value)
	}
}

// NeedsConfine reports whether policy requires a backend wrap.
func NeedsConfine(policy Policy) bool {
	return policy.Mode != "" && policy.Mode != ModeDangerFullAccess
}

// ValidatePolicy checks confined policies before Probe/Confine.
func ValidatePolicy(policy Policy) error {
	if !NeedsConfine(policy) {
		return nil
	}
	if strings.TrimSpace(policy.WorkspaceRoot) == "" {
		return errors.New("sandbox: workspace root is required")
	}
	switch policy.Mode {
	case ModeReadOnly, ModeWorkspaceWrite:
		return nil
	default:
		return fmt.Errorf("sandbox: invalid confined mode %q", policy.Mode)
	}
}
