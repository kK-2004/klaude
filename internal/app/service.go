package app

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kk-2004/klaude/internal/agent"
	"github.com/kk-2004/klaude/internal/approval"
	"github.com/kk-2004/klaude/internal/config"
	"github.com/kk-2004/klaude/internal/event"
	"github.com/kk-2004/klaude/internal/filesystem"
	gitservice "github.com/kk-2004/klaude/internal/git"
	"github.com/kk-2004/klaude/internal/project"
	"github.com/kk-2004/klaude/internal/secret"
	"github.com/kk-2004/klaude/internal/session"
	"github.com/kk-2004/klaude/internal/storage"
)

// Service 是面向桌面壳（Wails）的窄应用边界：只管生命周期与编排，
// 具体领域逻辑委托给 framework-neutral 的 internal 包。
type Service struct {
	logger      *slog.Logger
	mu          sync.RWMutex
	ready       bool
	data        storage.DataDirs
	db          *storage.DB
	projects    *project.Manager
	sessions    *session.Manager
	config      config.LoadResult
	startTurn   func(context.Context, string, string) error
	approvals   *approval.Manager
	undone      map[string]bool
	locks       *agent.MutationLocks
	secrets     secret.Store
	composition *Composition
	appContext  context.Context
}

func NewService(logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{logger: logger, approvals: approval.NewManager(), undone: make(map[string]bool), locks: agent.NewMutationLocks(), secrets: secret.NewStore()}
}

func NewServiceWithDataDirs(logger *slog.Logger, dirs storage.DataDirs) *Service {
	service := NewService(logger)
	service.data = dirs
	return service
}

func (s *Service) SetTurnRunner(runner func(context.Context, string, string) error) {
	s.startTurn = runner
}

// Startup 初始化数据目录与 Composition（DB/会话/审批等），然后标记应用就绪。
func (s *Service) Startup(appContext context.Context) {
	ctx := context.Background()
	if s.data.Base == "" {
		if dirs, err := storage.DefaultDataDirs(); err == nil {
			s.data = dirs
		}
	}
	if s.data.Base != "" {
		composition, err := Build(ctx, s.data, s.logger)
		if err != nil {
			s.logger.Error("failed to initialize application composition", "error", err)
			return
		}
		s.composition = composition
		s.db, s.projects, s.sessions = composition.DB, composition.Projects, composition.Sessions
		s.config, s.approvals = composition.Config, composition.Approvals
		if s.startTurn == nil {
			s.startTurn = s.runTurn
		}
		// Wails lifecycle contexts carry an events bridge. Plain contexts used by
		// tests and headless callers must not be passed to runtime.EventsEmit.
		if appContext != nil && appContext.Value("events") != nil {
			bridge := NewEventBridge()
			bridge.Context = appContext
			composition.Events.Subscribe(bridge.Forward)
		}
	}
	s.mu.Lock()
	s.appContext = appContext
	s.ready = true
	s.mu.Unlock()
	s.logger.Info("Klaude application started")
}

func (s *Service) Shutdown(_ context.Context) {
	s.mu.Lock()
	s.ready = false
	s.appContext = nil
	s.mu.Unlock()
	s.logger.Info("Klaude application stopped")
	if s.composition != nil {
		_ = s.composition.Close()
	} else if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *Service) Health() HealthResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return HealthResponse{Ready: s.ready, Product: "Klaude", Version: "0.1.0", Platform: runtime.GOOS}
}

type HealthResponse struct {
	Ready    bool   `json:"ready"`
	Product  string `json:"product"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
}

func (s *Service) OpenProject(ctx context.Context, path string) (storage.Project, error) {
	if s.projects == nil {
		return storage.Project{}, errors.New("application is not initialized")
	}
	return s.projects.Open(ctx, path)
}

func (s *Service) ListProjects(ctx context.Context) ([]storage.Project, error) {
	if s.db == nil {
		return nil, errors.New("application is not initialized")
	}
	return s.db.ListProjects(ctx)
}

func (s *Service) RenameProject(ctx context.Context, projectID, name string) (storage.Project, error) {
	if s.db == nil {
		return storage.Project{}, errors.New("application is not initialized")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return storage.Project{}, errors.New("project name is required")
	}
	if len([]rune(name)) > 120 {
		return storage.Project{}, errors.New("project name is too long")
	}
	if err := s.db.RenameProject(ctx, projectID, name); err != nil {
		return storage.Project{}, err
	}
	return s.db.GetProject(ctx, projectID)
}

func (s *Service) SetProjectPinned(ctx context.Context, projectID string, pinned bool) (storage.Project, error) {
	if s.db == nil {
		return storage.Project{}, errors.New("application is not initialized")
	}
	if err := s.db.SetProjectPinned(ctx, projectID, pinned); err != nil {
		return storage.Project{}, err
	}
	return s.db.GetProject(ctx, projectID)
}

// DeleteProject 先取消该项目下仍在运行的 Agent，再删除项目及级联的会话历史。
func (s *Service) DeleteProject(ctx context.Context, projectID string) error {
	if s.db == nil || s.sessions == nil {
		return errors.New("application is not initialized")
	}
	sessions, err := s.sessions.List(ctx, projectID)
	if err != nil {
		return err
	}
	for _, item := range sessions {
		if s.sessions.Cancel(item.ID) {
			waitContext, cancel := context.WithTimeout(ctx, 2*time.Second)
			_ = s.sessions.Wait(waitContext, item.ID)
			cancel()
		}
	}
	return s.db.DeleteProject(ctx, projectID)
}

// RevealProject 只接受已登记的项目 ID，避免前端传入任意路径触发系统命令。
func (s *Service) RevealProject(ctx context.Context, projectID string) error {
	if s.db == nil {
		return errors.New("application is not initialized")
	}
	stored, err := s.db.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	return project.Reveal(ctx, stored.RootPath)
}

// MoveSession 把草稿会话改挂到另一个项目，让重选项目沿用当前对话而不是新建。
func (s *Service) MoveSession(ctx context.Context, sessionID, projectID string) (storage.Session, error) {
	if s.db == nil {
		return storage.Session{}, errors.New("application is not initialized")
	}
	if _, err := s.db.GetProject(ctx, projectID); err != nil {
		return storage.Session{}, err
	}
	if err := s.db.MoveSession(ctx, sessionID, projectID); err != nil {
		return storage.Session{}, err
	}
	return s.db.GetSession(ctx, sessionID)
}

func (s *Service) CreateSession(ctx context.Context, projectID, title, providerName, modelName string) (storage.Session, error) {
	if s.sessions == nil {
		return storage.Session{}, errors.New("application is not initialized")
	}
	if title == "" {
		title = "New session"
	}
	if providerName == "" {
		providerName = s.config.Config.Provider.Name
	}
	if modelName == "" {
		modelName = s.config.Config.Provider.Model
	}
	if strings.TrimSpace(modelName) == "" {
		return storage.Session{}, errors.New("请先在设置中配置模型")
	}
	return s.sessions.Create(ctx, projectID, title, providerName, modelName)
}

func (s *Service) ListSessions(ctx context.Context, projectID string) ([]storage.Session, error) {
	if s.sessions == nil {
		return nil, errors.New("application is not initialized")
	}
	return s.sessions.List(ctx, projectID)
}

func (s *Service) ListRecentSessions(ctx context.Context) ([]storage.Session, error) {
	if s.db == nil {
		return nil, errors.New("application is not initialized")
	}
	return s.db.ListRecentSessions(ctx, 10)
}

func (s *Service) RenameSession(ctx context.Context, sessionID, title string) error {
	if s.db == nil {
		return errors.New("application is not initialized")
	}
	return s.db.RenameSession(ctx, sessionID, title)
}

func (s *Service) Settings() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Config
}

// SettingsUpdate is the desktop settings patch persisted to the user config.toml.
type SettingsUpdate struct {
	Theme              string `json:"theme"`
	Endpoint           string `json:"endpoint"`
	Model              string `json:"model"`
	CredentialEnv      string `json:"credentialEnv"`
	ContextBudgetChars int    `json:"contextBudgetChars"`
	MaxTurns           int    `json:"maxTurns"`
	ParallelTools      bool   `json:"parallelTools"`
	LLMSchedule        bool   `json:"llmSchedule"`
	ApprovalMode       string `json:"approvalMode"`
}

var credentialEnvPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// UpdateSettings applies the complete desktop settings form and writes the user config file.
func (s *Service) UpdateSettings(_ context.Context, update SettingsUpdate) (config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.config.Config
	if update.Theme == "" {
		update.Theme = cfg.UI.Theme
	}
	if strings.TrimSpace(update.Endpoint) == "" {
		update.Endpoint = cfg.Provider.Endpoint
	}
	if strings.TrimSpace(update.Model) == "" {
		update.Model = cfg.Provider.Model
	}
	if strings.TrimSpace(update.CredentialEnv) == "" && cfg.Provider.CredentialKey == "" {
		update.CredentialEnv = cfg.Provider.CredentialEnv
	}
	if update.ContextBudgetChars == 0 {
		update.ContextBudgetChars = cfg.Agent.ContextBudgetChars
	}
	if update.MaxTurns == 0 {
		update.MaxTurns = cfg.Agent.MaxTurns
	}
	if update.ApprovalMode == "" {
		update.ApprovalMode = approvalModeFromConfig(cfg)
	}
	if update.Theme != "light" && update.Theme != "dark" && update.Theme != "system" {
		return config.Config{}, errors.New("theme must be light, dark, or system")
	}
	update.Endpoint = strings.TrimSpace(update.Endpoint)
	update.Model = strings.TrimSpace(update.Model)
	update.CredentialEnv = strings.TrimSpace(update.CredentialEnv)
	if update.Endpoint == "" {
		return config.Config{}, errors.New("provider endpoint is required")
	}
	if update.Model != "" && update.CredentialEnv == "" && cfg.Provider.CredentialKey == "" {
		return config.Config{}, errors.New("provider credential is required")
	}
	if update.CredentialEnv != "" && !credentialEnvPattern.MatchString(update.CredentialEnv) {
		return config.Config{}, errors.New("credential environment variable is invalid")
	}
	if update.ContextBudgetChars <= 0 || update.MaxTurns <= 0 {
		return config.Config{}, errors.New("context budget and maximum turns must be greater than zero")
	}
	cfg.UI.Theme = update.Theme
	cfg.Provider.Endpoint = update.Endpoint
	cfg.Provider.Model = update.Model
	cfg.Provider.CredentialEnv = update.CredentialEnv
	if update.CredentialEnv != "" {
		cfg.Provider.CredentialKey = ""
	}
	if cfg.Provider.Model == "" {
		cfg.DefaultModel = ""
	} else {
		cfg.DefaultModel = cfg.Provider.Name + ":" + cfg.Provider.Model
	}
	cfg.Agent.ContextBudgetChars = update.ContextBudgetChars
	cfg.Agent.MaxTurns = update.MaxTurns
	cfg.Agent.ParallelTools = update.ParallelTools
	cfg.Agent.LLMSchedule = update.LLMSchedule && update.ParallelTools
	switch update.ApprovalMode {
	case "ask":
		cfg.Permissions.Read, cfg.Permissions.Write, cfg.Permissions.Shell, cfg.Permissions.Network = "allow", "ask", "ask", "ask"
	case "manual":
		cfg.Permissions.Read, cfg.Permissions.Write, cfg.Permissions.Shell, cfg.Permissions.Network = "ask", "ask", "ask", "ask"
	case "full":
		cfg.Permissions.Read, cfg.Permissions.Write, cfg.Permissions.Shell, cfg.Permissions.Network = "allow", "allow", "allow", "allow"
	default:
		return config.Config{}, errors.New("approval mode must be ask, manual, or full")
	}
	if err := config.Validate(cfg, false); err != nil {
		return config.Config{}, err
	}
	path := config.UserConfigPath(s.data.Base)
	if path == "" || s.data.Base == "" {
		return config.Config{}, errors.New("user config path is unavailable")
	}
	if err := config.Save(path, cfg); err != nil {
		return config.Config{}, err
	}
	s.config.Config = cfg
	if s.composition != nil {
		s.composition.Config.Config = cfg
		s.composition.Permissions = permissionsFromConfig(cfg)
		s.composition.Context.BudgetChars = cfg.Agent.ContextBudgetChars
		s.composition.Context.ToolResultChars = cfg.Agent.ToolResultChars
	}
	return cfg, nil
}

func approvalModeFromConfig(cfg config.Config) string {
	if cfg.Permissions.Read == "allow" && cfg.Permissions.Write == "allow" && cfg.Permissions.Shell == "allow" && cfg.Permissions.Network == "allow" {
		return "full"
	}
	if cfg.Permissions.Read == "ask" {
		return "manual"
	}
	return "ask"
}

// ConversationSnapshot 是桌面端断线重连用的只读快照：仅含已持久化数据，
// 事件序列出现空洞时可用它重新对齐 UI 状态。
type ConversationSnapshot struct {
	Session  storage.Session     `json:"session"`
	Messages []storage.Message   `json:"messages"`
	Turns    []storage.AgentTurn `json:"turns"`
}

func (s *Service) LoadConversation(ctx context.Context, sessionID string) (ConversationSnapshot, error) {
	if s.db == nil {
		return ConversationSnapshot{}, errors.New("application is not initialized")
	}
	session, err := s.db.GetSession(ctx, sessionID)
	if err != nil {
		return ConversationSnapshot{}, err
	}
	messages, err := s.db.ListMessages(ctx, sessionID)
	if err != nil {
		return ConversationSnapshot{}, err
	}
	turns, err := s.db.ListTurns(ctx, sessionID)
	if err != nil {
		return ConversationSnapshot{}, err
	}
	return ConversationSnapshot{Session: session, Messages: messages, Turns: turns}, nil
}

// SendMessage 持久化用户消息并创建新 turn，然后在独立 goroutine 中启动 Agent。
// 用独立 Background context，避免 RPC 请求结束导致长跑任务被取消。
func (s *Service) SendMessage(ctx context.Context, sessionID, content, providerName, modelName string) (storage.AgentTurn, error) {
	if s.db == nil || s.sessions == nil {
		return storage.AgentTurn{}, errors.New("application is not initialized")
	}
	if providerName == "" {
		providerName = s.config.Config.Provider.Name
	}
	if modelName == "" {
		modelName = s.config.Config.Provider.Model
	}
	if strings.TrimSpace(modelName) == "" {
		return storage.AgentTurn{}, errors.New("请先在设置中配置模型")
	}
	if err := s.db.UpdateSessionProviderModel(ctx, sessionID, providerName, modelName); err != nil {
		return storage.AgentTurn{}, err
	}
	_, turn, err := s.db.CreateTurnWithUserMessage(ctx, sessionID, content, providerName, modelName)
	if err != nil {
		return storage.AgentTurn{}, err
	}
	if s.startTurn != nil {
		runContext, cancel := context.WithCancel(context.Background())
		if err := s.sessions.RegisterActive(sessionID, cancel); err != nil {
			_ = s.db.UpdateTurnStatus(ctx, turn.ID, storage.TurnFailed, "active_runtime", err.Error())
			return storage.AgentTurn{}, err
		}
		go func() {
			defer s.sessions.Done(sessionID)
			if runErr := s.startTurn(runContext, sessionID, turn.ID); runErr != nil {
				_ = s.db.UpdateTurnStatus(context.Background(), turn.ID, storage.TurnFailed, "runtime_error", runErr.Error())
				if s.composition != nil && s.composition.Events != nil {
					_ = s.composition.Events.Publish(context.Background(), sessionID, turn.ID, event.AgentError, map[string]string{"code": "runtime_error", "message": runErr.Error()})
				}
			}
		}()
	}
	return turn, nil
}

func (s *Service) CancelAgent(sessionID string) bool {
	if s.sessions == nil {
		return false
	}
	return s.sessions.Cancel(sessionID)
}

type ApprovalResolution struct {
	ApprovalID  string `json:"approvalId"`
	Status      string `json:"status"`
	RequestHash string `json:"requestHash"`
}

func (s *Service) ResolveApproval(_ context.Context, request ApprovalResolution) error {
	if s.approvals == nil {
		return errors.New("approval manager is not initialized")
	}
	status := approval.Status(request.Status)
	return s.approvals.Resolve(approval.Resolution{ApprovalID: request.ApprovalID, Status: status, RequestHash: request.RequestHash})
}

func (s *Service) BrowseProject(ctx context.Context, root, target string) ([]filesystem.Entry, error) {
	service, err := filesystem.New(root)
	if err != nil {
		return nil, err
	}
	return service.ListDirectory(ctx, target)
}

func (s *Service) GitBranches(ctx context.Context, root string) (gitservice.BranchSnapshot, error) {
	return gitservice.New(root).Branches(ctx)
}

func (s *Service) CheckoutGitBranch(ctx context.Context, root, name string, remote bool) error {
	return gitservice.New(root).CheckoutBranch(ctx, name, remote)
}

func (s *Service) DeleteGitBranch(ctx context.Context, root, name string, remote bool) error {
	return gitservice.New(root).DeleteBranch(ctx, name, remote)
}

func (s *Service) CreateGitWorktree(ctx context.Context, root, startRef, branchName, targetPath string) (string, error) {
	return gitservice.New(root).CreateWorktree(ctx, startRef, branchName, targetPath)
}

func (s *Service) runtimeContext() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.appContext == nil {
		return context.Background()
	}
	return s.appContext
}

func (s *Service) Capabilities(ctx context.Context) []project.Capability {
	return project.ProbeCapabilities(ctx, s.config.Config.Provider.CredentialEnv)
}

func (s *Service) GetTurnChanges(ctx context.Context, turnID string) ([]storage.FileChange, error) {
	if s.db == nil {
		return nil, errors.New("application is not initialized")
	}
	return s.db.ListFileChanges(ctx, turnID)
}

// UndoTurn 按该 turn 记录的文件变更，用 before 快照把工作区回滚到变更前。
// 进程内 undone 表防止同一 turn 被重复撤销。
func (s *Service) UndoTurn(ctx context.Context, turnID string) error {
	if s.db == nil {
		return errors.New("application is not initialized")
	}
	if s.undone[turnID] {
		return errors.New("turn already undone")
	}
	root, err := s.db.ProjectRootForTurn(ctx, turnID)
	if err != nil {
		return err
	}
	dbChanges, err := s.db.ListFileChanges(ctx, turnID)
	if err != nil {
		return err
	}
	workspace, err := filesystem.New(root)
	if err != nil {
		return err
	}
	snapshots, err := filesystem.NewSnapshotStore(s.data.Snapshots)
	if err != nil {
		return err
	}
	changes := make([]filesystem.Change, 0, len(dbChanges))
	for _, item := range dbChanges {
		before := filesystem.Snapshot{Exists: item.BeforeHash != "", Hash: item.BeforeHash, ContentPath: filepath.Join(s.data.Snapshots, item.BeforeHash)}
		changes = append(changes, filesystem.Change{Path: item.Path, Status: item.Status, BeforeHash: item.BeforeHash, AfterHash: item.AfterHash, Before: before, Diff: item.Diff, AddedLines: item.AddedLines, DeletedLines: item.DeletedLines})
	}
	if err := workspace.Undo(snapshots, changes); err != nil {
		return err
	}
	s.undone[turnID] = true
	return nil
}
