package executor

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/klaude/klaude/internal/sandbox"
)

func TestLocalExecutorReturnsOutputAndExitCode(t *testing.T) {
	program, args := "printf", []string{"hello"}
	if runtime.GOOS == "windows" {
		program, args = "cmd", []string{"/C", "echo hello"}
	}
	result, err := (Local{}).Execute(context.Background(), Request{Program: program, Args: args, MaxOutput: 100})
	if err != nil || result.ExitCode != 0 || result.Stdout == "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestLocalExecutorTimesOut(t *testing.T) {
	program, args := "sleep", []string{"2"}
	if runtime.GOOS == "windows" {
		t.Skip("process timeout fixture differs on Windows")
	}
	result, err := (Local{}).Execute(context.Background(), Request{Program: program, Args: args, Timeout: 10 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) || !result.TimedOut {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestLocalExecutorFailClosedWithoutBackend(t *testing.T) {
	policy := sandbox.Policy{Mode: sandbox.ModeWorkspaceWrite, WorkspaceRoot: t.TempDir()}
	_, err := (Local{}).Execute(context.Background(), Request{
		Program: "printf", Args: []string{"x"}, Sandbox: &policy, MaxOutput: 100,
	})
	var unavailable *sandbox.UnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("expected UnavailableError, got %v", err)
	}
}

type passthroughBackend struct{}

func (passthroughBackend) Name() string { return "passthrough" }
func (passthroughBackend) Probe(context.Context) (bool, sandbox.Enforcement, error) {
	return true, sandbox.EnforcementFull, nil
}
func (passthroughBackend) Confine(_ context.Context, program string, args []string, _ sandbox.Policy) (sandbox.Confined, error) {
	return sandbox.Confined{Program: program, Args: args, Enforcement: sandbox.EnforcementFull, Backend: "passthrough"}, nil
}

func TestLocalExecutorAppliesSandboxPolicy(t *testing.T) {
	program, args := "printf", []string{"sandboxed"}
	if runtime.GOOS == "windows" {
		program, args = "cmd", []string{"/C", "echo sandboxed"}
	}
	policy := sandbox.Policy{Mode: sandbox.ModeReadOnly, WorkspaceRoot: t.TempDir()}
	result, err := (Local{Sandbox: passthroughBackend{}}).Execute(context.Background(), Request{
		Program: program, Args: args, Sandbox: &policy, MaxOutput: 100,
	})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if result.SandboxBackend != "passthrough" || result.SandboxOutcome != sandbox.OutcomeOK {
		t.Fatalf("sandbox fields=%+v", result)
	}
}

var _ = exec.ErrNotFound
