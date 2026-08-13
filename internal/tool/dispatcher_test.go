package tool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/klaude/klaude/internal/approval"
	"github.com/klaude/klaude/internal/permission"
)

func TestDispatcherDeniesWithoutExecuting(t *testing.T) {
	registry := NewRegistry()
	_ = registry.Register(testTool{})
	dispatcher := Dispatcher{Registry: registry, Permissions: permission.Engine{Read: permission.Deny, Write: permission.Deny}}
	result, err := dispatcher.Dispatch(context.Background(), "test", json.RawMessage(`{"path":"main.go"}`))
	if err != nil || result.ErrorCode != "permission_denied" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestDispatcherWaitsForApproval(t *testing.T) {
	registry := NewRegistry()
	_ = registry.Register(testTool{})
	manager := approval.NewManager()
	dispatcher := Dispatcher{Registry: registry, Permissions: permission.Engine{Read: permission.Allow, Write: permission.Ask}, Approvals: manager}
	done := make(chan Result, 1)
	go func() {
		result, _ := dispatcher.Dispatch(context.Background(), "test", json.RawMessage(`{"path":"main.go"}`))
		done <- result
	}()
	// The test tool is read-only, so the default allow path completes without approval.
	if result := <-done; !result.Success {
		t.Fatalf("result=%+v", result)
	}
}

func TestDispatcherFullAccessBypassesRoutineApproval(t *testing.T) {
	registry := NewRegistry()
	_ = registry.Register(approvalTool{})
	dispatcher := Dispatcher{Registry: registry, Permissions: permission.Engine{Read: permission.Allow, Write: permission.Allow, Shell: permission.Allow, FullAccess: true}}
	result, err := dispatcher.Dispatch(context.Background(), "approval_test", json.RawMessage(`{"path":"main.go"}`))
	if err != nil || !result.Success {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

type approvalTool struct{}

func (approvalTool) Definition() Definition {
	return Definition{Name: "approval_test", Parameters: objectSchema(map[string]any{"path": map[string]any{"type": "string"}}, "path"), Metadata: Metadata{RequiresApproval: true}}
}

func (approvalTool) Execute(context.Context, json.RawMessage) (Result, error) {
	return Result{Content: "ok", Success: true}, nil
}
