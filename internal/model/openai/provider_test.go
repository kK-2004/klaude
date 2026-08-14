package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kk-2004/klaude/internal/config"
	"github.com/kk-2004/klaude/internal/model"
)

func TestProviderParsesStreamingTextAndFragmentedToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("authorization header was not sent")
		}
		var payload struct {
			Tools []struct {
				Type     string               `json:"type"`
				Function model.ToolDefinition `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(payload.Tools) != 1 || payload.Tools[0].Type != "function" || payload.Tools[0].Function.Name != "read_file" {
			t.Errorf("tools = %+v", payload.Tools)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"))
		_, _ = writer.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","function":{"name":"read_file","arguments":"{\"path\":\"ma"}}]}}]}

`))
		_, _ = writer.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"in.go\"}"}}]}}]}

`))
		_, _ = writer.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	if err := os.Setenv("KLAUDE_TEST_OPENAI", "test-secret"); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("KLAUDE_TEST_OPENAI")
	provider, err := New(config.ProviderConfig{Endpoint: server.URL, Model: "test", CredentialEnv: "KLAUDE_TEST_OPENAI", AllowHTTPForLocal: true})
	if err != nil {
		t.Fatal(err)
	}
	events, err := provider.Stream(context.Background(), model.Request{
		Messages: []model.Message{{Role: model.RoleUser, Content: "go"}},
		Tools:    []model.ToolDefinition{{Name: "read_file", Description: "Read a file", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var text string
	var toolEnd model.Event
	for item := range events {
		if item.Type == model.TextDelta {
			text += item.Text
		}
		if item.Type == model.ToolCallEnd {
			toolEnd = item
		}
	}
	if text != "hello" || toolEnd.ID != "call-1" || toolEnd.Name != "read_file" || !strings.Contains(string(toolEnd.Arguments), "main.go") {
		t.Fatalf("text=%q tool=%+v", text, toolEnd)
	}
}

func TestProviderMissingCredentialDoesNotMakeNetworkRequest(t *testing.T) {
	provider, err := New(config.ProviderConfig{Endpoint: "https://example.com/v1", Model: "test", CredentialEnv: "KLAUDE_MISSING_PROVIDER"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Stream(context.Background(), model.Request{}); err == nil {
		t.Fatal("expected missing credential")
	}
}

func TestProviderResponsesAPIParsesTextToolsAndUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/responses" {
			t.Errorf("path = %q", request.URL.Path)
		}
		var payload struct {
			Model string `json:"model"`
			Store bool   `json:"store"`
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "responses-model" || payload.Store || len(payload.Tools) != 1 || payload.Tools[0].Name != "read_file" {
			t.Errorf("payload = %+v", payload)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\n"))
		_, _ = writer.Write([]byte("data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\",\"id\":\"item-1\",\"call_id\":\"call-1\",\"name\":\"read_file\"}}\n\n"))
		_, _ = writer.Write([]byte("data: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"item-1\",\"delta\":\"{\\\"path\\\":\\\"main.go\\\"}\"}\n\n"))
		_, _ = writer.Write([]byte("data: {\"type\":\"response.function_call_arguments.done\",\"item_id\":\"item-1\",\"arguments\":\"{\\\"path\\\":\\\"main.go\\\"}\"}\n\n"))
		_, _ = writer.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":3,\"output_tokens\":5}}}\n\n"))
	}))
	defer server.Close()
	provider, err := New(config.ProviderConfig{Endpoint: server.URL + "/v1", Model: "responses-model", APIKey: "secret", APIMode: "responses", AllowHTTPForLocal: true})
	if err != nil {
		t.Fatal(err)
	}
	events, err := provider.Stream(context.Background(), model.Request{Messages: []model.Message{{Role: model.RoleUser, Content: "test"}}, Tools: []model.ToolDefinition{{Name: "read_file", Parameters: map[string]any{"type": "object"}}}})
	if err != nil {
		t.Fatal(err)
	}
	var text string
	var call model.Event
	var completed bool
	for event := range events {
		switch event.Type {
		case model.TextDelta:
			text += event.Text
		case model.ToolCallEnd:
			call = event
		case model.ModelCompleted:
			completed = true
		}
	}
	if text != "OK" || call.ID != "call-1" || !strings.Contains(string(call.Arguments), "main.go") || !completed {
		t.Fatalf("text=%q call=%+v completed=%v", text, call, completed)
	}
}

func TestNormalizeHTTPError(t *testing.T) {
	response := &http.Response{StatusCode: http.StatusTooManyRequests, Body: http.NoBody}
	err := normalizeHTTPError(response)
	providerErr, ok := err.(*model.Error)
	if !ok || !providerErr.Retryable || providerErr.Code != "http_error" {
		t.Fatalf("error = %+v", err)
	}
}
