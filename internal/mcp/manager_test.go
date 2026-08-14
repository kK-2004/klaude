package mcp

import (
	"context"
	"testing"
)

func TestManagerPersistsDefinitionsAndFailureState(t *testing.T) {
	manager := NewManager(nil, nil)
	definition, err := manager.Upsert(Definition{Name: "Local", Transport: TransportStdio, Command: ""})
	if err == nil || definition.ID != "" {
		t.Fatal("expected invalid stdio definition to be rejected")
	}
	definition, err = manager.Upsert(Definition{ID: "local", Name: "Local", Transport: TransportStdio, Command: "definitely-not-a-real-klaude-command"})
	if err != nil {
		t.Fatal(err)
	}
	if got := manager.Snapshots(); len(got) != 1 || got[0].Status != "disconnected" || got[0].ID != definition.ID {
		t.Fatalf("snapshots = %+v", got)
	}
	if _, err := manager.Connect(context.Background(), definition.ID); err == nil {
		t.Fatal("expected stdio connection to fail")
	}
	if got := manager.Snapshots(); len(got) != 1 || got[0].Status != "error" || got[0].Error == "" {
		t.Fatalf("failure snapshot = %+v", got)
	}
}

func TestNamespacedToolName(t *testing.T) {
	got := NamespacedToolName(Definition{ID: "my server"}, "read_file")
	if got != "mcp__my_server__read_file" {
		t.Fatalf("namespaced tool name = %q", got)
	}
}
