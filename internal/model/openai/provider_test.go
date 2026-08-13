package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/klaude/klaude/internal/config"
	"github.com/klaude/klaude/internal/model"
)

func TestProviderParsesStreamingTextAndFragmentedToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("authorization header was not sent")
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
	events, err := provider.Stream(context.Background(), model.Request{Messages: []model.Message{{Role: model.RoleUser, Content: "go"}}})
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

func TestNormalizeHTTPError(t *testing.T) {
	response := &http.Response{StatusCode: http.StatusTooManyRequests, Body: http.NoBody}
	err := normalizeHTTPError(response)
	providerErr, ok := err.(*model.Error)
	if !ok || !providerErr.Retryable || providerErr.Code != "http_error" {
		t.Fatalf("error = %+v", err)
	}
}
