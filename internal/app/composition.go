package app

import (
	"context"
	"log/slog"

	"github.com/klaude/klaude/internal/approval"
	"github.com/klaude/klaude/internal/config"
	agentcontext "github.com/klaude/klaude/internal/context"
	"github.com/klaude/klaude/internal/event"
	"github.com/klaude/klaude/internal/executor"
	"github.com/klaude/klaude/internal/permission"
	"github.com/klaude/klaude/internal/project"
	"github.com/klaude/klaude/internal/session"
	"github.com/klaude/klaude/internal/storage"
	"github.com/klaude/klaude/internal/tool"
	"github.com/klaude/klaude/internal/trace"
)

// Composition 聚合应用启动后共享的基础设施与领域服务，供 Service 注入使用。
type Composition struct {
	Data        storage.DataDirs
	DB          *storage.DB
	Config      config.LoadResult
	Events      *event.Bus
	Approvals   *approval.Manager
	Permissions permission.Engine
	Executor    executor.Local
	Projects    *project.Manager
	Sessions    *session.Manager
	Tools       *tool.Registry
	Context     agentcontext.Manager
	Logger      *slog.Logger
	Trace       *trace.Writer
}

// Build 打开数据目录与 SQLite，恢复未完成的 in-flight turn，再装配默认依赖图。
func Build(ctx context.Context, dirs storage.DataDirs, logger *slog.Logger) (*Composition, error) {
	if err := dirs.Ensure(); err != nil {
		return nil, err
	}
	db, err := storage.Open(ctx, dirs.Database)
	if err != nil {
		return nil, err
	}
	// 上次进程崩溃留下的 running/waiting 状态在此收口为 interrupted。
	if !db.ReadOnly {
		if err := db.RecoverInFlight(ctx); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	loaded := config.Load(config.UserConfigPath(dirs.Base), "")
	composition := &Composition{Data: dirs, DB: db, Config: loaded, Events: event.NewBus(), Approvals: approval.NewManager(), Permissions: permission.NewDefault(), Executor: executor.Local{}, Projects: project.NewManager(db), Sessions: session.NewManager(db), Tools: tool.NewRegistry(), Context: agentcontext.Manager{SystemInstructions: "You are Klaude, a careful coding agent.", BudgetChars: loaded.Config.Agent.ContextBudgetChars, ToolResultChars: loaded.Config.Agent.ToolResultChars}, Logger: logger}
	tracePath := dirs.Traces + "/startup.jsonl"
	if writer, traceErr := trace.Open(tracePath, 10*1024*1024); traceErr == nil {
		composition.Trace = writer
	}
	return composition, nil
}

func (c *Composition) Close() error {
	if c.Trace != nil {
		_ = c.Trace.Close()
	}
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}
