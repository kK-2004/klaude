package executor

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

type Request struct {
	Program    string
	Args       []string
	WorkingDir string
	Env        map[string]string
	Timeout    time.Duration
	MaxOutput  int
}
type Result struct {
	Stdout    string
	Stderr    string
	ExitCode  int
	TimedOut  bool
	Truncated bool
	Duration  time.Duration
}
type Local struct{}

// Execute 在超时与输出上限约束下跑本地进程；超时后 terminate 整组进程。
func (Local) Execute(ctx context.Context, request Request) (Result, error) {
	if strings.TrimSpace(request.Program) == "" {
		return Result{}, errors.New("executor: program is empty")
	}
	if request.Timeout <= 0 {
		request.Timeout = 120 * time.Second
	}
	if request.MaxOutput <= 0 {
		request.MaxOutput = 24_000
	}
	commandContext, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()
	command := exec.CommandContext(commandContext, request.Program, request.Args...)
	command.Dir = request.WorkingDir
	configureProcess(command)
	for key, value := range request.Env {
		command.Env = append(command.Env, key+"="+value)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	started := time.Now()
	err := command.Run()
	duration := time.Since(started)
	if commandContext.Err() != nil {
		terminate(command)
		return Result{Stdout: limit(stdout.String(), request.MaxOutput), Stderr: limit(stderr.String(), request.MaxOutput), ExitCode: -1, TimedOut: errors.Is(commandContext.Err(), context.DeadlineExceeded), Truncated: len(stdout.String()) > request.MaxOutput || len(stderr.String()) > request.MaxOutput, Duration: duration}, commandContext.Err()
	}
	result := Result{Stdout: limit(stdout.String(), request.MaxOutput), Stderr: limit(stderr.String(), request.MaxOutput), ExitCode: command.ProcessState.ExitCode(), Truncated: len(stdout.String()) > request.MaxOutput || len(stderr.String()) > request.MaxOutput, Duration: duration}
	if err != nil {
		return result, err
	}
	return result, nil
}

func limit(value string, max int) string {
	if len(value) <= max {
		return value
	}
	marker := "...[truncated]..."
	if max <= len(marker) {
		return marker[:max]
	}
	head := max / 2
	tail := max - head - len(marker)
	return value[:head] + marker + value[len(value)-tail:]
}
