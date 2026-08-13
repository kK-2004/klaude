package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/klaude/klaude/internal/config"
	"github.com/klaude/klaude/internal/model"
)

// Provider 实现 OpenAI-compatible /chat/completions 流式协议，
// 把 SSE chunk 归一成 model.Event（文本增量、工具调用拼装、usage、完成）。
type Provider struct {
	Endpoint      string
	Model         string
	CredentialEnv string
	APIKey        string
	APIMode       string
	HTTPClient    *http.Client
	MaxTokens     int
	Temperature   *float64
	AllowHTTP     bool
}

func New(cfg config.ProviderConfig) (*Provider, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("openai provider: endpoint is empty")
	}
	// 默认强制 HTTPS；仅显式允许本地 HTTP 开发端点。
	if !strings.HasPrefix(cfg.Endpoint, "https://") && !(cfg.AllowHTTPForLocal && isLocalHTTP(cfg.Endpoint)) {
		return nil, errors.New("openai provider: endpoint must use https")
	}
	if cfg.Model == "" {
		return nil, errors.New("openai provider: model is empty")
	}
	mode := cfg.APIMode
	if mode == "" {
		mode = "chat_completions"
	}
	if mode != "chat_completions" && mode != "responses" {
		return nil, errors.New("openai provider: API mode must be chat_completions or responses")
	}
	var temperature *float64
	if cfg.Temperature >= 0 {
		value := cfg.Temperature
		temperature = &value
	}
	return &Provider{Endpoint: strings.TrimRight(cfg.Endpoint, "/"), Model: cfg.Model, CredentialEnv: cfg.CredentialEnv, APIKey: cfg.APIKey, APIMode: mode, HTTPClient: http.DefaultClient, MaxTokens: cfg.MaxOutputTokens, Temperature: temperature}, nil
}

func isLocalHTTP(endpoint string) bool {
	return strings.HasPrefix(endpoint, "http://localhost") || strings.HasPrefix(endpoint, "http://127.0.0.1") || strings.HasPrefix(endpoint, "http://[::1]")
}

type requestBody struct {
	Model       string          `json:"model"`
	Messages    []model.Message `json:"messages"`
	Tools       []openAITool    `json:"tools,omitempty"`
	Stream      bool            `json:"stream"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
}

type openAITool struct {
	Type     string               `json:"type"`
	Function model.ToolDefinition `json:"function"`
}

type sseChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			Role      string `json:"role"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error,omitempty"`
}

type assembledCall struct {
	id, name string
	args     bytes.Buffer
}

func (p *Provider) Stream(ctx context.Context, request model.Request) (<-chan model.Event, error) {
	credential, err := p.credential()
	if err != nil {
		return nil, &model.Error{Code: "missing_credential", Message: err.Error(), Cause: err}
	}
	modelName := request.Model
	if modelName == "" {
		modelName = p.Model
	}
	if p.APIMode == "responses" {
		return p.streamResponses(ctx, request, modelName, credential)
	}
	temperature := request.Temperature
	if temperature == nil {
		temperature = p.Temperature
	}
	body, err := json.Marshal(requestBody{Model: modelName, Messages: request.Messages, Tools: openAITools(request.Tools), Stream: true, MaxTokens: chooseMax(request.MaxTokens, p.MaxTokens), Temperature: temperature})
	if err != nil {
		return nil, &model.Error{Code: "encode_request", Message: "could not encode model request", Cause: err}
	}
	httpClient := p.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, &model.Error{Code: "request_build", Message: "could not build provider request", Cause: err}
	}
	httpRequest.Header.Set("Authorization", "Bearer "+credential)
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(httpRequest)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, model.ErrCancelled
		}
		return nil, &model.Error{Code: "network", Message: "model request failed", Retryable: true, Cause: err}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		return nil, normalizeHTTPError(response)
	}
	stream := make(chan model.Event, 16)
	go func() { defer close(stream); parseStream(ctx, response.Body, stream) }()
	return stream, nil
}

func (p *Provider) credential() (string, error) {
	if strings.TrimSpace(p.APIKey) != "" {
		return strings.TrimSpace(p.APIKey), nil
	}
	return config.ResolveCredential(p.CredentialEnv)
}

func openAITools(definitions []model.ToolDefinition) []openAITool {
	result := make([]openAITool, 0, len(definitions))
	for _, definition := range definitions {
		result = append(result, openAITool{Type: "function", Function: definition})
	}
	return result
}

// parseStream 按 index 拼装增量 tool_calls；收到 [DONE] 时补发尚未结束的 ToolCallEnd。
func parseStream(ctx context.Context, body io.ReadCloser, output chan<- model.Event) {
	defer body.Close()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024), 4*1024*1024)
	calls := make(map[int]*assembledCall)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			for _, call := range calls {
				emit(ctx, output, model.Event{Type: model.ToolCallEnd, ID: call.id, Name: call.name, Arguments: append([]byte(nil), call.args.Bytes()...)})
			}
			emit(ctx, output, model.Event{Type: model.ModelCompleted})
			return
		}
		var chunk sseChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return
		}
		if chunk.Error != nil {
			return
		}
		if chunk.Usage != nil {
			input, outputTokens := chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens
			emit(ctx, output, model.Event{Type: model.UsageUpdate, InputTokens: &input, OutputTokens: &outputTokens})
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				emit(ctx, output, model.Event{Type: model.TextDelta, Text: choice.Delta.Content})
			}
			for _, tool := range choice.Delta.ToolCalls {
				call := calls[tool.Index]
				if call == nil {
					call = &assembledCall{id: tool.ID, name: tool.Function.Name}
					calls[tool.Index] = call
					emit(ctx, output, model.Event{Type: model.ToolCallStart, ID: call.id, Name: call.name})
				}
				if call.id == "" {
					call.id = tool.ID
				}
				if call.name == "" {
					call.name = tool.Function.Name
				}
				if tool.Function.Arguments != "" {
					call.args.WriteString(tool.Function.Arguments)
					emit(ctx, output, model.Event{Type: model.ToolCallDelta, ID: call.id, Data: tool.Function.Arguments})
				}
			}
		}
	}
}

func emit(ctx context.Context, output chan<- model.Event, item model.Event) {
	select {
	case output <- item:
	case <-ctx.Done():
	}
}

func normalizeHTTPError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
	message := "provider returned HTTP " + strconv.Itoa(response.StatusCode)
	var payload sseChunk
	if json.Unmarshal(body, &payload) == nil && payload.Error != nil && payload.Error.Message != "" {
		message = payload.Error.Message
	}
	// 429 / 5xx 可重试；401 单独标为 authentication。
	retryable := response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
	if response.StatusCode == http.StatusUnauthorized {
		return &model.Error{Code: "authentication", Message: "provider authentication failed", Cause: fmt.Errorf("status %d", response.StatusCode)}
	}
	return &model.Error{Code: "http_error", Message: message, Retryable: retryable, Cause: fmt.Errorf("status %d", response.StatusCode)}
}

func chooseMax(requested, configured int) int {
	if requested > 0 {
		return requested
	}
	return configured
}

func RetryAfter(response *http.Response) time.Duration {
	seconds, err := strconv.Atoi(response.Header.Get("Retry-After"))
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
