package app

import (
	"context"
	"log/slog"
	"sort"

	"github.com/kk-2004/klaude/internal/approval"
	"github.com/kk-2004/klaude/internal/config"
	agentcontext "github.com/kk-2004/klaude/internal/context"
	"github.com/kk-2004/klaude/internal/event"
	"github.com/kk-2004/klaude/internal/executor"
	"github.com/kk-2004/klaude/internal/mcp"
	"github.com/kk-2004/klaude/internal/permission"
	"github.com/kk-2004/klaude/internal/project"
	"github.com/kk-2004/klaude/internal/sandbox"
	"github.com/kk-2004/klaude/internal/session"
	"github.com/kk-2004/klaude/internal/storage"
	"github.com/kk-2004/klaude/internal/tool"
	"github.com/kk-2004/klaude/internal/trace"
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
	MCP         *mcp.Manager
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
	composition := &Composition{
		Data:        dirs,
		DB:          db,
		Config:      loaded,
		Events:      event.NewBus(),
		Approvals:   approval.NewManager(),
		Permissions: permissionsFromConfig(loaded.Config),
		Executor:    executor.Local{Sandbox: sandbox.Platform()},
		Projects:    project.NewManager(db),
		Sessions:    session.NewManager(db),
		Tools:       tool.NewRegistry(),
		MCP:         mcp.NewManager(loaded.Config.MCPServers, logger),
		Context:     agentcontext.Manager{SystemInstructions: "You are Klaude, a careful coding agent.", BudgetChars: loaded.Config.Agent.ContextBudgetChars, ToolResultChars: loaded.Config.Agent.ToolResultChars},
		Logger:      logger,
	}
	tracePath := dirs.Traces + "/startup.jsonl"
	if writer, traceErr := trace.Open(tracePath, 10*1024*1024); traceErr == nil {
		composition.Trace = writer
	}
	return composition, nil
}

func permissionsFromConfig(cfg config.Config) permission.Engine {
	engine := permission.Engine{
		Read:    permission.Decision(cfg.Permissions.Read),
		Write:   permission.Decision(cfg.Permissions.Write),
		Shell:   permission.Decision(cfg.Permissions.Shell),
		Network: permission.Decision(cfg.Permissions.Network),
	}
	patterns := make([]string, 0, len(cfg.Permissions.ShellRules))
	for pattern := range cfg.Permissions.ShellRules {
		patterns = append(patterns, pattern)
	}
	sort.Strings(patterns)
	for _, pattern := range patterns {
		engine.Rules = append(engine.Rules, permission.Rule{Pattern: pattern, Decision: permission.Decision(cfg.Permissions.ShellRules[pattern])})
	}
	engine.FullAccess = engine.Read == permission.Allow && engine.Write == permission.Allow && engine.Shell == permission.Allow && engine.Network == permission.Allow
	return engine
}

func (c *Composition) Close() error {
	if c.MCP != nil {
		_ = c.MCP.Close()
	}
	if c.Trace != nil {
		_ = c.Trace.Close()
	}
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}
