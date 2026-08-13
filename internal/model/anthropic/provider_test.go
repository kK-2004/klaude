package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klaude/klaude/internal/config"
	"github.com/klaude/klaude/internal/model"
)

func TestProviderUsesMessagesProtocolAndParsesStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/messages" || request.Header.Get("x-api-key") != "secret" || request.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("request path=%q headers=%v", request.URL.Path, request.Header)
		}
		var payload struct {
			System string `json:"system"`
			Tools  []struct {
				Name        string         `json:"name"`
				InputSchema map[string]any `json:"input_schema"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.System != "system" || len(payload.Tools) != 1 || payload.Tools[0].Name != "read_file" || payload.Tools[0].InputSchema["type"] != "object" {
			t.Errorf("payload = %+v", payload)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":4}}}\n\n"))
		_, _ = writer.Write([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"OK\"}}\n\n"))
		_, _ = writer.Write([]byte("data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"tool_use\",\"id\":\"call-1\",\"name\":\"read_file\",\"input\":{}}}\n\n"))
		_, _ = writer.Write([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\\\"main.go\\\"}\"}}\n\n"))
		_, _ = writer.Write([]byte("data: {\"type\":\"content_block_stop\"}\n\n"))
		_, _ = writer.Write([]byte("data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":7}}\n\n"))
		_, _ = writer.Write([]byte("data: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()
	provider, err := New(config.ProviderConfig{Endpoint: server.URL + "/v1", Model: "claude-test", APIKey: "secret", APIMode: "messages", MaxOutputTokens: 100, AllowHTTPForLocal: true})
	if err != nil {
		t.Fatal(err)
	}
	events, err := provider.Stream(context.Background(), model.Request{Messages: []model.Message{{Role: model.RoleSystem, Content: "system"}, {Role: model.RoleUser, Content: "test"}}, Tools: []model.ToolDefinition{{Name: "read_file", Parameters: map[string]any{"type": "object"}}}})
	if err != nil {
		t.Fatal(err)
	}
	var text, args string
	var completed bool
	for event := range events {
		if event.Type == model.TextDelta {
			text += event.Text
		}
		if event.Type == model.ToolCallDelta {
			args += event.Data
		}
		if event.Type == model.ModelCompleted {
			completed = true
		}
	}
	if text != "OK" || !strings.Contains(args, "main.go") || !completed {
		t.Fatalf("text=%q args=%q completed=%v", text, args, completed)
	}
}
