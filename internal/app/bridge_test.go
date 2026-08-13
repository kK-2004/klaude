package app

import (
	"context"
	"testing"

	"github.com/klaude/klaude/internal/event"
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
