package event

import (
	"context"
	"testing"
)

func BenchmarkPublishEvent(b *testing.B) {
	bus := NewBus()
	bus.Subscribe(func(context.Context, Envelope) error { return nil })
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := bus.Publish(context.Background(), "session", "turn", TextDelta, map[string]string{"text": "delta"}); err != nil {
			b.Fatal(err)
		}
	}
}
