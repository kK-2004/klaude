package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/kk-2004/klaude/internal/config"
	"github.com/kk-2004/klaude/internal/model"
)

type Provider struct {
	Endpoint      string
	Model         string
	CredentialEnv string
	APIKey        string
	MaxTokens     int
	Temperature   *float64
	HTTPClient    *http.Client
}

func New(cfg config.ProviderConfig) (*Provider, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" || strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("anthropic provider: endpoint and model are required")
	}
	if !strings.HasPrefix(cfg.Endpoint, "https://") && !(cfg.AllowHTTPForLocal && isLocalHTTP(cfg.Endpoint)) {
		return nil, errors.New("anthropic provider: endpoint must use https")
	}
	value := cfg.Temperature
	return &Provider{Endpoint: strings.TrimRight(cfg.Endpoint, "/"), Model: cfg.Model, CredentialEnv: cfg.CredentialEnv, APIKey: cfg.APIKey, MaxTokens: cfg.MaxOutputTokens, Temperature: &value, HTTPClient: http.DefaultClient}, nil
}

func isLocalHTTP(endpoint string) bool {
	return strings.HasPrefix(endpoint, "http://localhost") || strings.HasPrefix(endpoint, "http://127.0.0.1") || strings.HasPrefix(endpoint, "http://[::1]")
}

type toolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type requestBody struct {
	Model       string           `json:"model"`
	System      string           `json:"system,omitempty"`
	Messages    []map[string]any `json:"messages"`
	Tools       []toolDefinition `json:"tools,omitempty"`
	Stream      bool             `json:"stream"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature *float64         `json:"temperature,omitempty"`
}

func (p *Provider) Stream(ctx context.Context, request model.Request) (<-chan model.Event, error) {
	credential := strings.TrimSpace(p.APIKey)
	var err error
	if credential == "" {
		credential, err = config.ResolveCredential(p.CredentialEnv)
		if err != nil {
			return nil, &model.Error{Code: "missing_credential", Message: err.Error(), Cause: err}
		}
	}
	modelName := request.Model
	if modelName == "" {
		modelName = p.Model
	}
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = p.MaxTokens
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	temperature := request.Temperature
	if temperature == nil {
		temperature = p.Temperature
	}
	system, messages := anthropicMessages(request.Messages)
	tools := make([]toolDefinition, 0, len(request.Tools))
	for _, item := range request.Tools {
		tools = append(tools, toolDefinition{Name: item.Name, Description: item.Description, InputSchema: item.Parameters})
	}
	encoded, err := json.Marshal(requestBody{Model: modelName, System: system, Messages: messages, Tools: tools, Stream: true, MaxTokens: maxTokens, Temperature: temperature})
	if err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint+"/messages", bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("x-api-key", credential)
	httpRequest.Header.Set("anthropic-version", "2023-06-01")
	httpRequest.Header.Set("content-type", "application/json")
	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, model.ErrCancelled
		}
		return nil, &model.Error{Code: "network", Message: "Anthropic Messages request failed", Retryable: true, Cause: err}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
		return nil, &model.Error{Code: "http_error", Message: strings.TrimSpace(string(data)), Retryable: response.StatusCode == 429 || response.StatusCode >= 500}
	}
	stream := make(chan model.Event, 16)
	go parseStream(ctx, response.Body, stream)
	return stream, nil
}

func anthropicMessages(messages []model.Message) (string, []map[string]any) {
	var system []string
	result := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		if message.Role == model.RoleSystem {
			system = append(system, message.Content)
			continue
		}
		if message.Role == model.RoleTool && message.ToolCallID != "" {
			result = append(result, map[string]any{"role": "user", "content": []map[string]any{{"type": "tool_result", "tool_use_id": message.ToolCallID, "content": message.Content}}})
			continue
		}
		result = append(result, map[string]any{"role": string(message.Role), "content": message.Content})
	}
	return strings.Join(system, "\n\n"), result
}

type streamEvent struct {
	Type    string `json:"type"`
	Message struct {
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	} `json:"message"`
	ContentBlock struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func parseStream(ctx context.Context, body io.ReadCloser, output chan<- model.Event) {
	defer close(output)
	defer body.Close()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024), 4*1024*1024)
	var activeID, activeName string
	var inputTokens, outputTokens int
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event streamEvent
		if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event) != nil {
			continue
		}
		switch event.Type {
		case "message_start":
			inputTokens = event.Message.Usage.InputTokens
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				activeID, activeName = event.ContentBlock.ID, event.ContentBlock.Name
				emit(ctx, output, model.Event{Type: model.ToolCallStart, ID: activeID, Name: activeName})
			}
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				emit(ctx, output, model.Event{Type: model.TextDelta, Text: event.Delta.Text})
			}
			if event.Delta.Type == "input_json_delta" {
				emit(ctx, output, model.Event{Type: model.ToolCallDelta, ID: activeID, Name: activeName, Data: event.Delta.PartialJSON})
			}
		case "content_block_stop":
			if activeID != "" {
				emit(ctx, output, model.Event{Type: model.ToolCallEnd, ID: activeID, Name: activeName})
				activeID, activeName = "", ""
			}
		case "message_delta":
			outputTokens = event.Usage.OutputTokens
		case "message_stop":
			emit(ctx, output, model.Event{Type: model.UsageUpdate, InputTokens: &inputTokens, OutputTokens: &outputTokens})
			emit(ctx, output, model.Event{Type: model.ModelCompleted})
			return
		}
	}
}

func emit(ctx context.Context, output chan<- model.Event, event model.Event) {
	select {
	case output <- event:
	case <-ctx.Done():
	}
}
