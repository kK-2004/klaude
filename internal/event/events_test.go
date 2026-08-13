package event

import (
	"context"
	"testing"
)

func TestBusSequencesEventsPerTurnAndUnsubscribes(t *testing.T) {
	bus := NewBus()
	var events []Envelope
	unsub := bus.Subscribe(func(_ context.Context, envelope Envelope) error { events = append(events, envelope); return nil })
	if err := bus.Publish(context.Background(), "s", "t", AgentStarted, nil); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(context.Background(), "s", "t", TextDelta, map[string]string{"text": "hi"}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("events = %+v", events)
	}
	unsub()
	if err := bus.Publish(context.Background(), "s", "t", AgentFinished, nil); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatal("subscriber was not removed")
	}
}
