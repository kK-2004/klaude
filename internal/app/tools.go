package app

import (
	"context"
	"time"

	"github.com/klaude/klaude/internal/agent"
	"github.com/klaude/klaude/internal/filesystem"
	"github.com/klaude/klaude/internal/storage"
	"github.com/klaude/klaude/internal/tool"
)

// ProjectTools is a per-turn / per-root tool registry with metadata lookup for the scheduler.
type ProjectTools struct {
	Registry *tool.Registry
	Lookup   agent.ToolMetaLookup
}

type registryLookup struct{ registry *tool.Registry }

func (r registryLookup) Lookup(name string) agent.ToolMeta {
	item, ok := r.registry.Get(name)
	if !ok {
		return agent.ToolMeta{Known: false}
	}
	meta := item.Definition().Metadata
	return agent.ToolMeta{ReadOnly: meta.ReadOnly, Concurrent: meta.Concurrent, Known: true}
}

// DBChangeRecorder persists filesystem changes for a turn.
type DBChangeRecorder struct {
	DB     *storage.DB
	TurnID string
}

func (r DBChangeRecorder) Record(ctx context.Context, change filesystem.Change) error {
	if r.DB == nil || r.TurnID == "" {
		return nil
	}
	return r.DB.AddFileChange(ctx, storage.FileChange{
		ID:           storage.NewID(),
		TurnID:       r.TurnID,
		Path:         change.Path,
		Status:       change.Status,
		BeforeHash:   change.BeforeHash,
		AfterHash:    change.AfterHash,
		Diff:         change.Diff,
		AddedLines:   change.AddedLines,
		DeletedLines: change.DeletedLines,
		CreatedAt:    time.Now().UTC(),
	})
}

// NewProjectTools builds read-only + mutating tools bound to a workspace root.
func (c *Composition) NewProjectTools(root, turnID string) (ProjectTools, error) {
	registry := tool.NewRegistry()
	workspace, err := filesystem.New(root)
	if err != nil {
		return ProjectTools{}, err
	}
	maxOutput := c.Config.Config.Agent.ToolResultChars
	if maxOutput <= 0 {
		maxOutput = 24_000
	}
	if err := tool.RegisterReadOnly(registry, tool.BuiltinContext{Workspace: workspace, MaxOutput: maxOutput}); err != nil {
		return ProjectTools{}, err
	}
	snapshots, err := filesystem.NewSnapshotStore(c.Data.Snapshots)
	if err != nil {
		return ProjectTools{}, err
	}
	shell := tool.NewShellTool(workspace.Root, c.Config.Config.Agent.ShellTimeoutSec, maxOutput, c.Executor)
	if err := tool.RegisterMutating(registry, tool.WriteContext{
		Workspace: workspace,
		Snapshots: snapshots,
		Recorder:  DBChangeRecorder{DB: c.DB, TurnID: turnID},
		MaxOutput: maxOutput,
	}, &shell); err != nil {
		return ProjectTools{}, err
	}
	return ProjectTools{Registry: registry, Lookup: registryLookup{registry: registry}}, nil
}

// SchedulerConfigFromApp maps user config into the agent scheduler flags.
func (c *Composition) SchedulerConfigFromApp() agent.SchedulerConfig {
	return agent.SchedulerConfig{
		ParallelTools: c.Config.Config.Agent.ParallelTools,
		LLMSchedule:   c.Config.Config.Agent.LLMSchedule,
	}
}
