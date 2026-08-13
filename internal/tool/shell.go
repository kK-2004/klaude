package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/klaude/klaude/internal/executor"
)

// ShellTool 在工作区执行有界非交互命令；默认 Destructive + RequiresApproval，须经审批。
type ShellTool struct {
	Workspace string
	Timeout   time.Duration
	MaxOutput int
	Executor  executor.Local
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
	result, err := t.Executor.Execute(ctx, executor.Request{Program: args.Program, Args: args.Args, WorkingDir: t.Workspace, Timeout: t.Timeout, MaxOutput: t.MaxOutput})
	content := result.Stdout
	if result.Stderr != "" {
		content += "\n[stderr]\n" + result.Stderr
	}
	toolResult := Result{Content: content, Success: err == nil && result.ExitCode == 0, Truncated: result.Truncated, Metadata: map[string]any{"exitCode": result.ExitCode, "durationMs": result.Duration.Milliseconds(), "timedOut": result.TimedOut}}
	if err != nil {
		if result.TimedOut {
			toolResult.ErrorCode = "timeout"
		} else if errors.Is(ctx.Err(), context.Canceled) {
			toolResult.ErrorCode = "cancelled"
		} else {
			toolResult.ErrorCode = "command_failed"
		}
		return toolResult, err
	}
	return toolResult, nil
}

var _ = exec.ErrNotFound
