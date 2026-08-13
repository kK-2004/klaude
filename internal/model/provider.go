package model

import (
	"context"
	"errors"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role   `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type Request struct {
	Model       string
	Messages    []Message
	Tools       []ToolDefinition
	MaxTokens   int
	Temperature *float64
	Metadata    map[string]string
}

type EventType string

const (
	TextDelta      EventType = "text_delta"
	ToolCallStart  EventType = "tool_call_start"
	ToolCallDelta  EventType = "tool_call_delta"
	ToolCallEnd    EventType = "tool_call_end"
	UsageUpdate    EventType = "usage"
	ModelCompleted EventType = "completed"
)

type Event struct {
	Type            EventType
	Text            string
	ID              string
	Name            string
	Data            string
	Arguments       []byte
	InputTokens     *int
	CachedTokens    *int
	OutputTokens    *int
	ReasoningTokens *int
}

type Error struct {
	Code      string
	Message   string
	Retryable bool
	Cause     error
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }
func (e *Error) Unwrap() error { return e.Cause }

var ErrCancelled = &Error{Code: "cancelled", Message: "model request cancelled"}

// Provider 是模型后端抽象：只暴露流式事件通道，便于 Agent 与具体厂商解耦。
type Provider interface {
	Stream(ctx context.Context, request Request) (<-chan Event, error)
}

func IsCancelled(err error) bool {
	var providerErr *Error
	return errors.As(err, &providerErr) && providerErr.Code == ErrCancelled.Code
}
