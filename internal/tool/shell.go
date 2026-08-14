package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/klaude/klaude/internal/executor"
	"github.com/klaude/klaude/internal/sandbox"
)

// ShellTool 在工作区执行有界非交互命令；默认 Destructive + RequiresApproval，须经审批。
type ShellTool struct {
	Workspace     string
	Timeout       time.Duration
	MaxOutput     int
	Executor      executor.Local
	SandboxPolicy *sandbox.Policy
}

func (t ShellTool) Definition() Definition {
	return Definition{Name: "shell", Description: "Run a bounded non-interactive command in the workspace", Parameters: map[string]any{"type": "object", "properties": map[string]any{"program": map[string]any{"type": "string"}, "args": map[string]any{"type": "array"}}, "required": []any{"program"}}, Metadata: Metadata{Destructive: true, RequiresApproval: true}}
}

func (t ShellTool) Execute(ctx context.Context, arguments json.RawMessage) (Result, error) {
	var args struct {
		Program string   `json:"program"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return Result{ErrorCode: "invalid_arguments"}, err
	}
	if strings.TrimSpace(args.Program) == "" {
		return Result{ErrorCode: "invalid_arguments"}, errors.New("program is empty")
	}
	if t.Timeout <= 0 {
		t.Timeout = 120 * time.Second
	}
	if t.MaxOutput <= 0 {
		t.MaxOutput = 24_000
	}
	request := executor.Request{
		Program:    args.Program,
		Args:       args.Args,
		WorkingDir: t.Workspace,
		Timeout:    t.Timeout,
		MaxOutput:  t.MaxOutput,
		Sandbox:    t.SandboxPolicy,
	}
	result, err := t.Executor.Execute(ctx, request)
	content := result.Stdout
	if result.Stderr != "" {
		content += "\n[stderr]\n" + result.Stderr
	}
	if result.SandboxOutcome == sandbox.OutcomeDenied && t.SandboxPolicy != nil {
		content += "\n" + sandbox.DenialMarker(t.SandboxPolicy.Mode)
	}
	if result.SandboxOutcome == sandbox.OutcomeRunnerFailed {
		content += "\n[sandbox: runner failed — command did not start under confinement]"
	}
	toolResult := Result{
		Content:   content,
		Success:   err == nil && result.ExitCode == 0 && result.SandboxOutcome != sandbox.OutcomeDenied && result.SandboxOutcome != sandbox.OutcomeRunnerFailed,
		Truncated: result.Truncated,
		Metadata: map[string]any{
			"exitCode":           result.ExitCode,
			"durationMs":         result.Duration.Milliseconds(),
			"timedOut":           result.TimedOut,
			"sandboxBackend":     result.SandboxBackend,
			"sandboxEnforcement": string(result.SandboxEnforce),
			"sandboxOutcome":     string(result.SandboxOutcome),
		},
	}
	if err != nil {
		var unavailable *sandbox.UnavailableError
		if errors.As(err, &unavailable) {
			toolResult.ErrorCode = "sandbox_unavailable"
			toolResult.Content = unavailable.Error()
			return toolResult, err
		}
		if result.TimedOut {
			toolResult.ErrorCode = "timeout"
		} else if errors.Is(ctx.Err(), context.Canceled) {
			toolResult.ErrorCode = "cancelled"
		} else if result.SandboxOutcome == sandbox.OutcomeDenied {
			toolResult.ErrorCode = "sandbox_denied"
		} else if result.SandboxOutcome == sandbox.OutcomeRunnerFailed {
			toolResult.ErrorCode = "sandbox_runner_failed"
		} else {
			toolResult.ErrorCode = "command_failed"
		}
		return toolResult, err
	}
	if result.SandboxOutcome == sandbox.OutcomeDenied {
		toolResult.ErrorCode = "sandbox_denied"
		return toolResult, nil
	}
	if result.SandboxOutcome == sandbox.OutcomeRunnerFailed {
		toolResult.ErrorCode = "sandbox_runner_failed"
		return toolResult, errors.New("sandbox runner failed")
	}
	return toolResult, nil
}

var _ = exec.ErrNotFound
