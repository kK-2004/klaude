package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/klaude/klaude/internal/executor"
	"github.com/klaude/klaude/internal/filesystem"
)

// ChangeRecorder persists a successful filesystem mutation for review/undo.
type ChangeRecorder interface {
	Record(ctx context.Context, change filesystem.Change) error
}

// WriteContext is shared by mutating file tools.
type WriteContext struct {
	Workspace filesystem.Service
	Snapshots filesystem.SnapshotStore
	Recorder  ChangeRecorder
	MaxOutput int
}

// RegisterMutating registers write_file, apply_patch, and optionally shell.
func RegisterMutating(registry *Registry, write WriteContext, shell *ShellTool) error {
	for _, item := range []Tool{WriteFileTool{write}, ApplyPatchTool{write}} {
		if err := registry.Register(item); err != nil {
			return err
		}
	}
	if shell != nil {
		if err := registry.Register(*shell); err != nil {
			return err
		}
	}
	return nil
}

type WriteFileTool struct{ Ctx WriteContext }

func (t WriteFileTool) Definition() Definition {
	return Definition{
		Name:        "write_file",
		Description: "Write a text file inside the workspace",
		Parameters: objectSchema(map[string]any{
			"path":          map[string]any{"type": "string"},
			"content":       map[string]any{"type": "string"},
			"expected_hash": map[string]any{"type": "string"},
		}, "path", "content"),
		Metadata: Metadata{RequiresApproval: true},
	}
}

func (t WriteFileTool) Execute(ctx context.Context, arguments json.RawMessage) (Result, error) {
	var args struct {
		Path         string `json:"path"`
		Content      string `json:"content"`
		ExpectedHash string `json:"expected_hash"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return Result{ErrorCode: "invalid_arguments"}, err
	}
	change, err := t.Ctx.Workspace.WriteFile(t.Ctx.Snapshots, args.Path, []byte(args.Content), args.ExpectedHash)
	if err != nil {
		return mapWriteError(err), err
	}
	if t.Ctx.Recorder != nil && change.Status != "no_change" {
		if err := t.Ctx.Recorder.Record(ctx, change); err != nil {
			return Result{ErrorCode: "record_failed", Content: err.Error()}, err
		}
	}
	return LimitResult(Result{
		Content: fmt.Sprintf("wrote %s (%s)", change.Path, change.Status),
		Success: true,
		Metadata: map[string]any{
			"path":       change.Path,
			"status":     change.Status,
			"beforeHash": change.BeforeHash,
			"afterHash":  change.AfterHash,
		},
	}, t.Ctx.MaxOutput), nil
}

type ApplyPatchTool struct{ Ctx WriteContext }

func (t ApplyPatchTool) Definition() Definition {
	return Definition{
		Name:        "apply_patch",
		Description: "Replace an exact text span inside a workspace file",
		Parameters: objectSchema(map[string]any{
			"path":          map[string]any{"type": "string"},
			"old_text":      map[string]any{"type": "string"},
			"new_text":      map[string]any{"type": "string"},
			"expected_hash": map[string]any{"type": "string"},
		}, "path", "old_text", "new_text"),
		Metadata: Metadata{RequiresApproval: true},
	}
}

func (t ApplyPatchTool) Execute(ctx context.Context, arguments json.RawMessage) (Result, error) {
	var args struct {
		Path         string `json:"path"`
		OldText      string `json:"old_text"`
		NewText      string `json:"new_text"`
		ExpectedHash string `json:"expected_hash"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return Result{ErrorCode: "invalid_arguments"}, err
	}
	change, err := t.Ctx.Workspace.ApplyPatch(t.Ctx.Snapshots, args.Path, args.OldText, args.NewText, args.ExpectedHash)
	if err != nil {
		return mapWriteError(err), err
	}
	if t.Ctx.Recorder != nil && change.Status != "no_change" {
		if err := t.Ctx.Recorder.Record(ctx, change); err != nil {
			return Result{ErrorCode: "record_failed", Content: err.Error()}, err
		}
	}
	return LimitResult(Result{
		Content: fmt.Sprintf("patched %s (%s)", change.Path, change.Status),
		Success: true,
		Metadata: map[string]any{
			"path":       change.Path,
			"status":     change.Status,
			"beforeHash": change.BeforeHash,
			"afterHash":  change.AfterHash,
			"added":      change.AddedLines,
			"deleted":    change.DeletedLines,
		},
	}, t.Ctx.MaxOutput), nil
}

func mapWriteError(err error) Result {
	switch {
	case errors.Is(err, filesystem.ErrOutsideWorkspace):
		return Result{ErrorCode: "workspace_boundary", Content: err.Error()}
	case errors.Is(err, filesystem.ErrStaleBaseline):
		return Result{ErrorCode: "stale_baseline", Content: err.Error()}
	case errors.Is(err, filesystem.ErrAmbiguousPatch):
		return Result{ErrorCode: "ambiguous_patch", Content: err.Error()}
	default:
		return Result{ErrorCode: "write_failed", Content: err.Error()}
	}
}

// NewShellTool builds a bounded workspace shell tool for registry registration.
func NewShellTool(workspace string, timeoutSec, maxOutput int, exec executor.Local) ShellTool {
	timeout := time.Duration(timeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	if maxOutput <= 0 {
		maxOutput = 24_000
	}
	return ShellTool{Workspace: workspace, Timeout: timeout, MaxOutput: maxOutput, Executor: exec}
}
