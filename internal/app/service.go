package app

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/klaude/klaude/internal/approval"
	"github.com/klaude/klaude/internal/config"
	"github.com/klaude/klaude/internal/filesystem"
	gitservice "github.com/klaude/klaude/internal/git"
	"github.com/klaude/klaude/internal/project"
	"github.com/klaude/klaude/internal/session"
	"github.com/klaude/klaude/internal/storage"
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
	composition *Composition
	appContext  context.Context
}

func NewService(logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{logger: logger, approvals: approval.NewManager(), undone: make(map[string]bool)}
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
	return HealthResponse{Ready: s.ready, Product: "Klaude", Version: "0.1.0"}
}

type HealthResponse struct {
	Ready   bool   `json:"ready"`
	Product string `json:"product"`
	Version string `json:"version"`
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
	return s.sessions.Create(ctx, projectID, title, providerName, modelName)
}

func (s *Service) ListSessions(ctx context.Context, projectID string) ([]storage.Session, error) {
	if s.sessions == nil {
		return nil, errors.New("application is not initialized")
	}
	return s.sessions.List(ctx, projectID)
}

func (s *Service) RenameSession(ctx context.Context, sessionID, title string) error {
	if s.db == nil {
		return errors.New("application is not initialized")
	}
	return s.db.RenameSession(ctx, sessionID, title)
}

func (s *Service) Settings() config.Config { return s.config.Config }

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
