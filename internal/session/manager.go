package session

import (
	"context"
	"errors"
	"sync"

	"github.com/kk-2004/klaude/internal/storage"
)

// Manager 管理会话 CRUD，并维护「每会话最多一个活跃 Agent」的 cancel 句柄表。
type Manager struct {
	DB     *storage.DB
	mu     sync.Mutex
	active map[string]activeRuntime
}

type activeRuntime struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func NewManager(db *storage.DB) *Manager {
	return &Manager{DB: db, active: make(map[string]activeRuntime)}
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
	m.active[sessionID] = activeRuntime{cancel: cancel, done: make(chan struct{})}
	return nil
}

// Cancel 触发该会话活跃 Agent 的 context 取消；返回是否找到活跃运行时。
func (m *Manager) Cancel(sessionID string) bool {
	m.mu.Lock()
	runtime, exists := m.active[sessionID]
	m.mu.Unlock()
	if exists {
		runtime.cancel()
	}
	return exists
}

// Wait 等待指定会话的 Agent 真正退出。删除项目时需要先等它结束，避免后台
// goroutine 在级联删除后继续写入 SQLite。
func (m *Manager) Wait(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	runtime, exists := m.active[sessionID]
	m.mu.Unlock()
	if !exists {
		return nil
	}
	select {
	case <-runtime.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) Done(sessionID string) {
	m.mu.Lock()
	runtime, exists := m.active[sessionID]
	if exists {
		delete(m.active, sessionID)
		close(runtime.done)
	}
	m.mu.Unlock()
}
