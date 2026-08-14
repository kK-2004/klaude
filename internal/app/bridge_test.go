package app

import (
	"context"
	"testing"

	"github.com/kk-2004/klaude/internal/event"
)

func TestEventBridgeUsesStableChannel(t *testing.T) {
	var channel string
	bridge := EventBridge{Publish: func(_ context.Context, name string, _ event.Envelope) error { channel = name; return nil }}
	if err := bridge.Forward(context.Background(), event.Envelope{TurnID: "turn"}); err != nil {
		t.Fatal(err)
	}
	if channel != AgentEventChannel {
		t.Fatalf("channel = %q", channel)
	}
}

func TestEventBridgeUsesLifecycleContext(t *testing.T) {
	lifecycleContext := context.WithValue(context.Background(), "events", struct{}{})
	var received context.Context
	bridge := EventBridge{
		Context: lifecycleContext,
		Publish: func(ctx context.Context, _ string, _ event.Envelope) error {
			received = ctx
			return nil
		},
	}

	if err := bridge.Forward(context.Background(), event.Envelope{TurnID: "turn"}); err != nil {
		t.Fatal(err)
	}
	if received != lifecycleContext {
		t.Fatalf("received context = %p, want lifecycle context %p", received, lifecycleContext)
	}
}
