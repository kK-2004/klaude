package executor

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"testing"
	"time"
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

var _ = exec.ErrNotFound
