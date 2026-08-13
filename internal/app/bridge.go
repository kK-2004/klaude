package app

import (
	"context"

	"github.com/klaude/klaude/internal/event"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AgentEventChannel 是前端订阅 Agent 实时事件的 Wails 频道名。
const AgentEventChannel = "klaude:agent-event"

// EventBridge 把内部 event.Bus 信封转发到 Wails EventsEmit。
type EventBridge struct {
	Publish func(context.Context, string, event.Envelope) error
}

func NewEventBridge() EventBridge {
	return EventBridge{Publish: func(ctx context.Context, channel string, envelope event.Envelope) error {
		runtime.EventsEmit(ctx, channel, envelope)
		return nil
	}}
}

func (b EventBridge) Forward(ctx context.Context, envelope event.Envelope) error {
	if b.Publish == nil {
		return nil
	}
	return b.Publish(ctx, AgentEventChannel, envelope)
}
