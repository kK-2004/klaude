package fake

import (
	"context"
	"testing"

	"github.com/klaude/klaude/internal/model"
)

func TestCodingTurnFixtureIsDeterministicAndOffline(t *testing.T) {
	provider := NewCodingTurn()
	stream, err := provider.Stream(context.Background(), model.Request{Model: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	var got []model.Event
	for event := range stream {
		got = append(got, event)
	}
	if len(got) != 5 || got[1].Type != model.ToolCallStart || got[3].Type != model.UsageUpdate {
		t.Fatalf("fixture events = %+v", got)
	}
	second, err := provider.Stream(context.Background(), model.Request{Model: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	var patchSeen bool
	for event := range second {
		patchSeen = patchSeen || event.Type == model.ToolCallStart && event.Name == "apply_patch"
	}
	if !patchSeen {
		t.Fatal("fixture did not emit the approved patch tool")
	}
}
