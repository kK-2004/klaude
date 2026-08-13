package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/klaude/klaude/internal/model"
)

type responsesBody struct {
	Model           string          `json:"model"`
	Input           []any           `json:"input"`
	Tools           []responsesTool `json:"tools,omitempty"`
	Stream          bool            `json:"stream"`
	Store           bool            `json:"store"`
	MaxOutputTokens int             `json:"max_output_tokens,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
}

type responsesTool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type responsesEvent struct {
	Type      string `json:"type"`
	Delta     string `json:"delta"`
	ItemID    string `json:"item_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Item      struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		CallID    string `json:"call_id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"item"`
	Response struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"response"`
}

func (p *Provider) streamResponses(ctx context.Context, request model.Request, modelName, credential string) (<-chan model.Event, error) {
	temperature := request.Temperature
	if temperature == nil {
		temperature = p.Temperature
	}
	body := responsesBody{Model: modelName, Input: responseInputs(request.Messages), Tools: responseTools(request.Tools), Stream: true, Store: false, MaxOutputTokens: chooseMax(request.MaxTokens, p.MaxTokens), Temperature: temperature}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, &model.Error{Code: "encode_request", Message: "could not encode Responses request", Cause: err}
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint+"/responses", bytes.NewReader(encoded))
	if err != nil {
		return nil, &model.Error{Code: "request_build", Message: "could not build Responses request", Cause: err}
	}
	httpRequest.Header.Set("Authorization", "Bearer "+credential)
	httpRequest.Header.Set("Content-Type", "application/json")
	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, model.ErrCancelled
		}
		return nil, &model.Error{Code: "network", Message: "Responses request failed", Retryable: true, Cause: err}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		return nil, normalizeHTTPError(response)
	}
	stream := make(chan model.Event, 16)
	go parseResponsesStream(ctx, response.Body, stream)
	return stream, nil
}

func responseInputs(messages []model.Message) []any {
	result := make([]any, 0, len(messages))
	for _, message := range messages {
		if message.Role == model.RoleTool && message.ToolCallID != "" {
			result = append(result, map[string]any{"type": "function_call_output", "call_id": message.ToolCallID, "output": message.Content})
			continue
		}
		role := string(message.Role)
		if message.Role == model.RoleSystem {
			role = "developer"
		}
		result = append(result, map[string]any{"role": role, "content": message.Content})
	}
	return result
}

func responseTools(definitions []model.ToolDefinition) []responsesTool {
	result := make([]responsesTool, 0, len(definitions))
	for _, definition := range definitions {
		result = append(result, responsesTool{Type: "function", Name: definition.Name, Description: definition.Description, Parameters: definition.Parameters})
	}
	return result
}

func parseResponsesStream(ctx context.Context, body io.ReadCloser, output chan<- model.Event) {
	defer close(output)
	defer body.Close()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024), 4*1024*1024)
	started := make(map[string]bool)
	callIDs := make(map[string]string)
	callNames := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var item responsesEvent
		if json.Unmarshal([]byte(data), &item) != nil {
			continue
		}
		switch item.Type {
		case "response.output_text.delta":
			emit(ctx, output, model.Event{Type: model.TextDelta, Text: item.Delta})
		case "response.output_item.added":
			if item.Item.Type == "function_call" {
				id := item.Item.CallID
				if id == "" {
					id = item.Item.ID
				}
				callIDs[item.Item.ID], callNames[item.Item.ID] = id, item.Item.Name
				started[id] = true
				emit(ctx, output, model.Event{Type: model.ToolCallStart, ID: id, Name: item.Item.Name})
			}
		case "response.function_call_arguments.delta":
			id, name := callIDs[item.ItemID], callNames[item.ItemID]
			if id == "" {
				id = item.ItemID
			}
			if name == "" {
				name = item.Name
			}
			if !started[id] {
				started[id] = true
				emit(ctx, output, model.Event{Type: model.ToolCallStart, ID: id, Name: name})
			}
			emit(ctx, output, model.Event{Type: model.ToolCallDelta, ID: id, Name: name, Data: item.Delta})
		case "response.function_call_arguments.done":
			id, name := callIDs[item.ItemID], callNames[item.ItemID]
			if id == "" {
				id = item.ItemID
			}
			if name == "" {
				name = item.Name
			}
			emit(ctx, output, model.Event{Type: model.ToolCallEnd, ID: id, Name: name, Arguments: []byte(item.Arguments)})
		case "response.completed":
			if item.Response.Usage != nil {
				input, outputTokens := item.Response.Usage.InputTokens, item.Response.Usage.OutputTokens
				emit(ctx, output, model.Event{Type: model.UsageUpdate, InputTokens: &input, OutputTokens: &outputTokens})
			}
			emit(ctx, output, model.Event{Type: model.ModelCompleted})
			return
		case "response.failed", "error":
			return
		}
	}
}
