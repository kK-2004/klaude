package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kk-2004/klaude/internal/config"
	"github.com/kk-2004/klaude/internal/filesystem"
	gitservice "github.com/kk-2004/klaude/internal/git"
	"github.com/kk-2004/klaude/internal/project"
	"github.com/kk-2004/klaude/internal/storage"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// RPCService 是 Wails 前端门面：方法参数需可 JSON 序列化，因此不暴露 context.Context；
// 内部改用 Background，取消走独立 CancelAgent RPC。DTO 映射也集中在此层。
type RPCService struct {
	service *Service
}

func NewRPCService(service *Service) *RPCService { return &RPCService{service: service} }

func (r *RPCService) Health() HealthResponse { return r.service.Health() }

func (r *RPCService) MCPServers() []MCPServerDTO { return r.service.MCPServers() }

func (r *RPCService) SaveMCPServer(input MCPServerInput) ([]MCPServerDTO, error) {
	return r.service.SaveMCPServer(context.Background(), input)
}

func (r *RPCService) DeleteMCPServer(id string) ([]MCPServerDTO, error) {
	return r.service.DeleteMCPServer(context.Background(), id)
}

func (r *RPCService) ConnectMCPServer(id string) ([]MCPServerDTO, error) {
	return r.service.ConnectMCPServer(context.Background(), id)
}

func (r *RPCService) DisconnectMCPServer(id string) ([]MCPServerDTO, error) {
	return r.service.DisconnectMCPServer(id)
}

type ProjectDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	RootPath  string `json:"rootPath"`
	GitRoot   string `json:"gitRoot,omitempty"`
	Pinned    bool   `json:"pinned"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type SessionDTO struct {
	ID        string                `json:"id"`
	ProjectID string                `json:"projectId"`
	Title     string                `json:"title"`
	Provider  string                `json:"provider"`
	Model     string                `json:"model"`
	Status    storage.SessionStatus `json:"status"`
	CreatedAt string                `json:"createdAt"`
	UpdatedAt string                `json:"updatedAt"`
}

type MessageDTO struct {
	ID        string              `json:"id"`
	SessionID string              `json:"sessionId"`
	TurnID    string              `json:"turnId,omitempty"`
	Role      storage.MessageRole `json:"role"`
	Content   string              `json:"content"`
	CreatedAt string              `json:"createdAt"`
}

type AgentTurnDTO struct {
	ID         string             `json:"id"`
	SessionID  string             `json:"sessionId"`
	Status     storage.TurnStatus `json:"status"`
	StartedAt  string             `json:"startedAt"`
	FinishedAt string             `json:"finishedAt,omitempty"`
	ErrorCode  string             `json:"errorCode,omitempty"`
	ErrorText  string             `json:"errorText,omitempty"`
}

type FileChangeDTO struct {
	ID           string `json:"id"`
	TurnID       string `json:"turnId"`
	ToolCallID   string `json:"toolCallId"`
	Path         string `json:"path"`
	Status       string `json:"status"`
	BeforeHash   string `json:"beforeHash"`
	AfterHash    string `json:"afterHash"`
	Diff         string `json:"diff"`
	AddedLines   int    `json:"addedLines"`
	DeletedLines int    `json:"deletedLines"`
	CreatedAt    string `json:"createdAt"`
}

type ConversationSnapshotDTO struct {
	Session  SessionDTO     `json:"session"`
	Messages []MessageDTO   `json:"messages"`
	Turns    []AgentTurnDTO `json:"turns"`
}

func (r *RPCService) ListProjects() ([]ProjectDTO, error) {
	projects, err := r.service.ListProjects(context.Background())
	return mapProjects(projects), err
}

func (r *RPCService) OpenProject(path string) (ProjectDTO, error) {
	project, err := r.service.OpenProject(context.Background(), path)
	return mapProject(project), err
}

func (r *RPCService) RenameProject(projectID, name string) (ProjectDTO, error) {
	renamed, err := r.service.RenameProject(context.Background(), projectID, name)
	return mapProject(renamed), err
}

func (r *RPCService) SetProjectPinned(projectID string, pinned bool) (ProjectDTO, error) {
	updated, err := r.service.SetProjectPinned(context.Background(), projectID, pinned)
	return mapProject(updated), err
}

func (r *RPCService) DeleteProject(projectID string) error {
	return r.service.DeleteProject(context.Background(), projectID)
}

func (r *RPCService) RevealProject(projectID string) error {
	return r.service.RevealProject(context.Background(), projectID)
}

func (r *RPCService) MoveSession(sessionID, projectID string) (SessionDTO, error) {
	moved, err := r.service.MoveSession(context.Background(), sessionID, projectID)
	return mapSession(moved), err
}

func (r *RPCService) CreateSession(projectID, title, providerName, modelName string) (SessionDTO, error) {
	session, err := r.service.CreateSession(context.Background(), projectID, title, providerName, modelName)
	return mapSession(session), err
}

func (r *RPCService) ListSessions(projectID string) ([]SessionDTO, error) {
	sessions, err := r.service.ListSessions(context.Background(), projectID)
	return mapSessions(sessions), err
}

func (r *RPCService) ListRecentSessions() ([]SessionDTO, error) {
	sessions, err := r.service.ListRecentSessions(context.Background())
	return mapSessions(sessions), err
}

func (r *RPCService) RenameSession(sessionID, title string) error {
	return r.service.RenameSession(context.Background(), sessionID, title)
}

func (r *RPCService) LoadConversation(sessionID string) (ConversationSnapshotDTO, error) {
	snapshot, err := r.service.LoadConversation(context.Background(), sessionID)
	if err != nil {
		return ConversationSnapshotDTO{}, err
	}
	return ConversationSnapshotDTO{Session: mapSession(snapshot.Session), Messages: mapMessages(snapshot.Messages), Turns: mapTurns(snapshot.Turns)}, nil
}

func (r *RPCService) SendMessage(sessionID, content, providerName, modelName string) (AgentTurnDTO, error) {
	turn, err := r.service.SendMessage(context.Background(), sessionID, content, providerName, modelName)
	return mapTurn(turn), err
}

func (r *RPCService) CancelAgent(sessionID string) bool { return r.service.CancelAgent(sessionID) }

func (r *RPCService) BrowseProject(root, target string) ([]filesystem.Entry, error) {
	return r.service.BrowseProject(context.Background(), root, target)
}

func (r *RPCService) GitBranches(root string) (gitservice.BranchSnapshot, error) {
	return r.service.GitBranches(context.Background(), root)
}

func (r *RPCService) CheckoutGitBranch(root, name string, remote bool) error {
	return r.service.CheckoutGitBranch(context.Background(), root, name, remote)
}

func (r *RPCService) DeleteGitBranch(root, name string, remote bool) error {
	return r.service.DeleteGitBranch(context.Background(), root, name, remote)
}

func (r *RPCService) CreateGitWorktree(root, startRef, branchName, targetPath string) (string, error) {
	return r.service.CreateGitWorktree(context.Background(), root, startRef, branchName, targetPath)
}

func (r *RPCService) SelectDirectory(defaultDirectory string) (string, error) {
	return runtime.OpenDirectoryDialog(r.service.runtimeContext(), runtime.OpenDialogOptions{
		DefaultDirectory: resolveDialogDirectory(defaultDirectory),
		Title:            "选择目录",
	})
}

func resolveDialogDirectory(candidate string) string {
	candidate = strings.TrimSpace(candidate)
	home, _ := os.UserHomeDir()
	if candidate == "~" {
		candidate = home
	} else if strings.HasPrefix(candidate, "~"+string(filepath.Separator)) && home != "" {
		candidate = filepath.Join(home, strings.TrimPrefix(candidate, "~"+string(filepath.Separator)))
	}

	if directoryExists(candidate) {
		if absolute, err := filepath.Abs(candidate); err == nil {
			return absolute
		}
		return filepath.Clean(candidate)
	}
	if directoryExists(home) {
		return home
	}
	return ""
}

func directoryExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (r *RPCService) Capabilities() []project.Capability {
	return r.service.Capabilities(context.Background())
}

func (r *RPCService) GetTurnChanges(turnID string) ([]FileChangeDTO, error) {
	changes, err := r.service.GetTurnChanges(context.Background(), turnID)
	return mapChanges(changes), err
}

func (r *RPCService) UndoTurn(turnID string) error {
	return r.service.UndoTurn(context.Background(), turnID)
}

func (r *RPCService) ResolveApproval(request ApprovalResolution) error {
	return r.service.ResolveApproval(context.Background(), request)
}

func (r *RPCService) Settings() config.Config { return r.service.Settings() }

func (r *RPCService) UpdateSettings(update SettingsUpdate) (config.Config, error) {
	return r.service.UpdateSettings(context.Background(), update)
}

func (r *RPCService) ModelProfiles() (ModelCatalog, error) {
	return r.service.ModelProfiles(context.Background())
}

func (r *RPCService) SaveModelProfile(input ModelProfileInput) (ModelCatalog, error) {
	return r.service.SaveModelProfile(context.Background(), input)
}

func (r *RPCService) SelectModelProfile(profileID string) (ModelCatalog, error) {
	return r.service.SelectModelProfile(context.Background(), profileID)
}

func (r *RPCService) TestModelConnection(input ModelProfileInput) (ModelConnectionResult, error) {
	return r.service.TestModelConnection(context.Background(), input)
}

func mapProject(project storage.Project) ProjectDTO {
	return ProjectDTO{ID: project.ID, Name: project.Name, RootPath: project.RootPath, GitRoot: project.GitRoot, Pinned: project.Pinned, CreatedAt: formatTime(project.CreatedAt), UpdatedAt: formatTime(project.UpdatedAt)}
}

func mapProjects(projects []storage.Project) []ProjectDTO {
	result := make([]ProjectDTO, 0, len(projects))
	for _, project := range projects {
		result = append(result, mapProject(project))
	}
	return result
}

func mapSession(session storage.Session) SessionDTO {
	return SessionDTO{ID: session.ID, ProjectID: session.ProjectID, Title: session.Title, Provider: session.Provider, Model: session.Model, Status: session.Status, CreatedAt: formatTime(session.CreatedAt), UpdatedAt: formatTime(session.UpdatedAt)}
}

func mapSessions(sessions []storage.Session) []SessionDTO {
	result := make([]SessionDTO, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, mapSession(session))
	}
	return result
}

func mapMessage(message storage.Message) MessageDTO {
	return MessageDTO{ID: message.ID, SessionID: message.SessionID, TurnID: message.TurnID, Role: message.Role, Content: message.Content, CreatedAt: formatTime(message.CreatedAt)}
}

func mapMessages(messages []storage.Message) []MessageDTO {
	result := make([]MessageDTO, 0, len(messages))
	for _, message := range messages {
		result = append(result, mapMessage(message))
	}
	return result
}

func mapTurn(turn storage.AgentTurn) AgentTurnDTO {
	result := AgentTurnDTO{ID: turn.ID, SessionID: turn.SessionID, Status: turn.Status, StartedAt: formatTime(turn.StartedAt), ErrorCode: turn.ErrorCode, ErrorText: turn.ErrorText}
	if turn.FinishedAt != nil {
		result.FinishedAt = formatTime(*turn.FinishedAt)
	}
	return result
}

func mapTurns(turns []storage.AgentTurn) []AgentTurnDTO {
	result := make([]AgentTurnDTO, 0, len(turns))
	for _, turn := range turns {
		result = append(result, mapTurn(turn))
	}
	return result
}

func mapChanges(changes []storage.FileChange) []FileChangeDTO {
	result := make([]FileChangeDTO, 0, len(changes))
	for _, change := range changes {
		result = append(result, FileChangeDTO{ID: change.ID, TurnID: change.TurnID, ToolCallID: change.ToolCallID, Path: change.Path, Status: change.Status, BeforeHash: change.BeforeHash, AfterHash: change.AfterHash, Diff: change.Diff, AddedLines: change.AddedLines, DeletedLines: change.DeletedLines, CreatedAt: formatTime(change.CreatedAt)})
	}
	return result
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
