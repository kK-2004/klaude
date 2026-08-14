package executor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kk-2004/klaude/internal/sandbox"
)

type Request struct {
	Program    string
	Args       []string
	WorkingDir string
	Env        map[string]string
	Timeout    time.Duration
	MaxOutput  int
	// Sandbox, when set with a confining mode, wraps argv before spawn.
	// danger_full_access or a nil policy runs unconfined.
	Sandbox *sandbox.Policy
}

type Result struct {
	Stdout         string
	Stderr         string
	ExitCode       int
	TimedOut       bool
	Truncated      bool
	Duration       time.Duration
	SandboxBackend string
	SandboxEnforce sandbox.Enforcement
	SandboxOutcome sandbox.Outcome
}

// Local executes subprocesses with optional OS sandbox wrapping.
type Local struct {
	Sandbox sandbox.Backend
}

// Execute 在超时与输出上限约束下跑本地进程；超时后 terminate 整组进程。
func (l Local) Execute(ctx context.Context, request Request) (Result, error) {
	if strings.TrimSpace(request.Program) == "" {
		return Result{}, errors.New("executor: program is empty")
	}
	if request.Timeout <= 0 {
		request.Timeout = 120 * time.Second
	}
	if request.MaxOutput <= 0 {
		request.MaxOutput = 24_000
	}

	program := request.Program
	args := append([]string(nil), request.Args...)
	envMap := cloneEnv(request.Env)
	var confined sandbox.Confined
	usedSandbox := false
	var command *exec.Cmd

	commandContext, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()

	if request.Sandbox != nil && sandbox.NeedsConfine(*request.Sandbox) {
		if l.Sandbox == nil {
			return Result{}, &sandbox.UnavailableError{Reason: "executor has no sandbox backend"}
		}
		usedSandbox = true
		if builder, ok := l.Sandbox.(sandbox.CommandBuilder); ok {
			baseEnv := os.Environ()
			if len(envMap) > 0 {
				baseEnv = mergeEnviron(baseEnv, envMap)
			}
			var err error
			command, confined, err = builder.PrepareCommand(commandContext, program, args, request.WorkingDir, baseEnv, *request.Sandbox)
			if err != nil {
				return Result{}, err
			}
			if confined.Release != nil {
				defer confined.Release()
			}
		} else {
			var err error
			confined, err = l.Sandbox.Confine(commandContext, program, args, *request.Sandbox)
			if err != nil {
				return Result{}, err
			}
			if confined.Release != nil {
				defer confined.Release()
			}
			program = confined.Program
			args = confined.Args
			for key, value := range confined.Env {
				if envMap == nil {
					envMap = map[string]string{}
				}
				envMap[key] = value
			}
			command = exec.CommandContext(commandContext, program, args...)
			command.Dir = request.WorkingDir
			configureProcess(command)
			if len(envMap) > 0 {
				command.Env = mergeEnviron(os.Environ(), envMap)
			}
		}
	} else {
		command = exec.CommandContext(commandContext, program, args...)
		command.Dir = request.WorkingDir
		configureProcess(command)
		if len(envMap) > 0 {
			command.Env = mergeEnviron(os.Environ(), envMap)
		}
	}

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	started := time.Now()
	err := command.Run()
	duration := time.Since(started)
	out := limit(stdout.String(), request.MaxOutput)
	errOut := limit(stderr.String(), request.MaxOutput)
	if commandContext.Err() != nil {
		terminate(command)
		result := Result{
			Stdout: out, Stderr: errOut, ExitCode: -1,
			TimedOut:  errors.Is(commandContext.Err(), context.DeadlineExceeded),
			Truncated: len(stdout.String()) > request.MaxOutput || len(stderr.String()) > request.MaxOutput,
			Duration:  duration,
		}
		if usedSandbox {
			result.SandboxBackend = confined.Backend
			result.SandboxEnforce = confined.Enforcement
			result.SandboxOutcome = sandbox.OutcomeCommandFailed
		}
		return result, commandContext.Err()
	}
	exitCode := 0
	if command.ProcessState != nil {
		exitCode = command.ProcessState.ExitCode()
	}
	result := Result{
		Stdout: out, Stderr: errOut, ExitCode: exitCode,
		Truncated: len(stdout.String()) > request.MaxOutput || len(stderr.String()) > request.MaxOutput,
		Duration:  duration,
	}
	if usedSandbox {
		result.SandboxBackend = confined.Backend
		result.SandboxEnforce = confined.Enforcement
		result.SandboxOutcome = sandbox.Classify(exitCode, stderr.String(), confined)
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

func cloneEnv(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func mergeEnviron(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	index := make(map[string]int, len(base))
	out := append([]string(nil), base...)
	for i, entry := range out {
		if key, _, ok := strings.Cut(entry, "="); ok {
			index[key] = i
		}
	}
	for key, value := range overrides {
		entry := key + "=" + value
		if i, ok := index[key]; ok {
			out[i] = entry
		} else {
			out = append(out, entry)
		}
	}
	return out
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
