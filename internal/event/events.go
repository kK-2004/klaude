package event

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Type string

const (
	AgentStarted     Type = "agent.started"
	TextDelta        Type = "agent.text_delta"
	ToolStarted      Type = "agent.tool_started"
	ToolFinished     Type = "agent.tool_finished"
	ApprovalRequired Type = "agent.approval_required"
	UsageUpdated     Type = "agent.usage"
	AgentError       Type = "agent.error"
	AgentCancelled   Type = "agent.cancelled"
	AgentFinished    Type = "agent.finished"
)

type Envelope struct {
	Version    int       `json:"version"`
	EventID    string    `json:"eventId"`
	Sequence   int64     `json:"sequence"`
	OccurredAt time.Time `json:"occurredAt"`
	ProjectID  string    `json:"projectId,omitempty"`
	SessionID  string    `json:"sessionId"`
	TurnID     string    `json:"turnId"`
	Type       Type      `json:"type"`
	Payload    any       `json:"payload,omitempty"`
}

type Sink interface {
	Emit(context.Context, Envelope) error
}

type Subscriber func(context.Context, Envelope) error

// Bus 按 TurnID 分配单调 Sequence，并向订阅者同步派发 Envelope。
// Sequence 供前端检测事件空洞并触发快照重拉。
type Bus struct {
	mu        sync.RWMutex
	subs      map[int]Subscriber
	nextSubID int
	seq       map[string]int64
}

func NewBus() *Bus { return &Bus{subs: make(map[int]Subscriber), seq: make(map[string]int64)} }

func (b *Bus) Subscribe(subscriber Subscriber) (unsubscribe func()) {
	b.mu.Lock()
	id := b.nextSubID
	b.nextSubID++
	b.subs[id] = subscriber
	b.mu.Unlock()
	return func() { b.mu.Lock(); delete(b.subs, id); b.mu.Unlock() }
}

func (b *Bus) Emit(ctx context.Context, envelope Envelope) error {
	b.mu.Lock()
	if envelope.Version == 0 {
		envelope.Version = 1
	}
	if envelope.EventID == "" {
		envelope.EventID = newID()
	}
	if envelope.OccurredAt.IsZero() {
		envelope.OccurredAt = time.Now().UTC()
	}
	if envelope.Sequence == 0 {
		b.seq[envelope.TurnID]++
		envelope.Sequence = b.seq[envelope.TurnID]
	}
	subs := make([]Subscriber, 0, len(b.subs))
	for _, subscriber := range b.subs {
		subs = append(subs, subscriber)
	}
	b.mu.Unlock()
	for _, subscriber := range subs {
		if err := subscriber(ctx, envelope); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bus) Publish(ctx context.Context, sessionID, turnID string, typ Type, payload any) error {
	return b.Emit(ctx, Envelope{SessionID: sessionID, TurnID: turnID, Type: typ, Payload: payload})
}

func newID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf[:])
}
