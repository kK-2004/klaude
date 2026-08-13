package tool

import (
	"context"
	"encoding/json"
	"testing"
)

type testTool struct{}

func (testTool) Definition() Definition {
	return Definition{Name: "test", Parameters: map[string]any{"required": []any{"path"}, "properties": map[string]any{"path": map[string]any{"type": "string"}}}, Metadata: Metadata{ReadOnly: true, Concurrent: true}}
}
func (testTool) Execute(context.Context, json.RawMessage) (Result, error) {
	return Result{Content: "ok", Success: true}, nil
}

func TestRegistryAndSchemaValidation(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(testTool{}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateArguments(testTool{}.Definition(), json.RawMessage(`{"path":"main.go"}`)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateArguments(testTool{}.Definition(), json.RawMessage(`{"path":1}`)); err == nil {
		t.Fatal("expected type error")
	}
	if err := registry.Register(testTool{}); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestLimitResult(t *testing.T) {
	result := LimitResult(Result{Content: "01234567890123456789", Success: true}, 10)
	if !result.Truncated || len(result.Content) != 10 {
		t.Fatalf("result = %+v", result)
	}
}
