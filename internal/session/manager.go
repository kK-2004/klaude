package session

import (
	"context"
	"errors"
	"sync"

	"github.com/klaude/klaude/internal/storage"
)

// Manager 管理会话 CRUD，并维护「每会话最多一个活跃 Agent」的 cancel 句柄表。
type Manager struct {
	DB     *storage.DB
	mu     sync.Mutex
	active map[string]context.CancelFunc
}

func NewManager(db *storage.DB) *Manager {
	return &Manager{DB: db, active: make(map[string]context.CancelFunc)}
}

func (m *Manager) Create(ctx context.Context, projectID, title, provider, model string) (storage.Session, error) {
	session := storage.Session{ID: storage.NewID(), ProjectID: projectID, Title: title, Provider: provider, Model: model, Status: storage.SessionIdle}
	if err := m.DB.CreateSession(ctx, session); err != nil {
		return storage.Session{}, err
	}
	return session, nil
}

func (m *Manager) List(ctx context.Context, projectID string) ([]storage.Session, error) {
	return m.DB.ListSessions(ctx, projectID)
}

func (m *Manager) RegisterActive(sessionID string, cancel context.CancelFunc) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.active[sessionID]; exists {
		return errors.New("session already has an active runtime")
	}
	m.active[sessionID] = cancel
	return nil
}

// Cancel 触发该会话活跃 Agent 的 context 取消；返回是否找到活跃运行时。
func (m *Manager) Cancel(sessionID string) bool {
	m.mu.Lock()
	cancel, exists := m.active[sessionID]
	if exists {
		delete(m.active, sessionID)
	}
	m.mu.Unlock()
	if exists {
		cancel()
	}
	return exists
}

func (m *Manager) Done(sessionID string) { m.mu.Lock(); delete(m.active, sessionID); m.mu.Unlock() }
