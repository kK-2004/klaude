package approval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

type Status string

const (
	Pending   Status = "pending"
	Approved  Status = "approved"
	Rejected  Status = "rejected"
	Cancelled Status = "cancelled"
	Expired   Status = "expired"
)

type Request struct {
	ID          string
	SessionID   string
	TurnID      string
	ToolCallID  string
	ToolName    string
	Summary     string
	WorkingDir  string
	Risk        string
	RequestHash string
}

type Resolution struct {
	ApprovalID  string
	Status      Status
	RequestHash string
}
type pending struct {
	request Request
	result  chan Resolution
}

// Manager 管理进程内待审批请求：Create 登记 → Wait 阻塞 → Resolve 按哈希校验后放行/拒绝。
type Manager struct {
	mu       sync.Mutex
	pending  map[string]*pending
	resolved map[string]Resolution
}

func NewManager() *Manager {
	return &Manager{pending: make(map[string]*pending), resolved: make(map[string]Resolution)}
}

// Hash 对审批摘要做 SHA-256，Resolve 时比对，防止前端改包后批准另一请求。
func Hash(summary string) string {
	digest := sha256.Sum256([]byte(summary))
	return hex.EncodeToString(digest[:])
}

func (m *Manager) Create(request Request) Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	if request.ID == "" {
		request.ID = newID()
	}
	if request.RequestHash == "" {
		request.RequestHash = Hash(request.Summary)
	}
	m.pending[request.ID] = &pending{request: request, result: make(chan Resolution, 1)}
	return request
}

// Wait 阻塞直到该审批被 Resolve，或 ctx 取消（记为 Cancelled）。
// 若在 Wait 前已 Resolve，则从 resolved 缓冲取出一次结果。
func (m *Manager) Wait(ctx context.Context, approvalID string) (Resolution, error) {
	m.mu.Lock()
	item, ok := m.pending[approvalID]
	if !ok {
		resolution, resolved := m.resolved[approvalID]
		if resolved {
			delete(m.resolved, approvalID)
			m.mu.Unlock()
			return resolution, nil
		}
	}
	m.mu.Unlock()
	if !ok {
		return Resolution{}, errors.New("approval is not pending")
	}
	select {
	case <-ctx.Done():
		m.finish(approvalID, Resolution{ApprovalID: approvalID, Status: Cancelled, RequestHash: item.request.RequestHash})
		return Resolution{}, ctx.Err()
	case resolution := <-item.result:
		return resolution, nil
	}
}

func (m *Manager) Resolve(resolution Resolution) error {
	m.mu.Lock()
	item, ok := m.pending[resolution.ApprovalID]
	if !ok {
		m.mu.Unlock()
		return errors.New("approval is not pending")
	}
	if resolution.RequestHash != item.request.RequestHash {
		m.mu.Unlock()
		return errors.New("approval request hash mismatch")
	}
	if resolution.Status != Approved && resolution.Status != Rejected && resolution.Status != Cancelled && resolution.Status != Expired {
		m.mu.Unlock()
		return errors.New("invalid approval resolution")
	}
	delete(m.pending, resolution.ApprovalID)
	m.resolved[resolution.ApprovalID] = resolution
	m.mu.Unlock()
	item.result <- resolution
	return nil
}

func (m *Manager) CancelAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.pending))
	for id := range m.pending {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Resolve(Resolution{ApprovalID: id, Status: Expired, RequestHash: m.hash(id)})
	}
}
func (m *Manager) hash(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item := m.pending[id]; item != nil {
		return item.request.RequestHash
	}
	return ""
}
func (m *Manager) finish(id string, resolution Resolution) { _ = m.Resolve(resolution) }
func newID() string                                        { return Hash(time.Now().UTC().String())[:32] }
