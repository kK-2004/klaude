package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Metadata 描述工具对权限/并发调度的约束，由 Dispatcher 与 Agent 批量逻辑消费。
type Metadata struct {
	ReadOnly         bool `json:"readOnly"`
	Destructive      bool `json:"destructive"`
	Concurrent       bool `json:"concurrent"`
	RequiresApproval bool `json:"requiresApproval"`
}

type Definition struct {
	Name        string
	Description string
	Parameters  map[string]any
	Metadata    Metadata
}

type Tool interface {
	Definition() Definition
	Execute(context.Context, json.RawMessage) (Result, error)
}

type Result struct {
	Content   string         `json:"content"`
	Success   bool           `json:"success"`
	ErrorCode string         `json:"errorCode,omitempty"`
	Truncated bool           `json:"truncated"`
	RawRef    string         `json:"rawRef,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type Registry struct{ tools map[string]Tool }

func NewRegistry() *Registry { return &Registry{tools: make(map[string]Tool)} }

func (r *Registry) Register(item Tool) error {
	if item == nil {
		return errors.New("tool: cannot register nil tool")
	}
	name := item.Definition().Name
	if strings.TrimSpace(name) == "" {
		return errors.New("tool: name is empty")
	}
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool: %s is already registered", name)
	}
	r.tools[name] = item
	return nil
}

func (r *Registry) Get(name string) (Tool, bool) { item, ok := r.tools[name]; return item, ok }

func (r *Registry) Definitions() []Definition {
	result := make([]Definition, 0, len(r.tools))
	for _, item := range r.tools {
		result = append(result, item.Definition())
	}
	return result
}

// ValidateArguments 按 JSON Schema 风格的 Parameters（required + properties.type）做轻量校验。
func ValidateArguments(def Definition, arguments json.RawMessage) error {
	var object map[string]any
	if err := json.Unmarshal(arguments, &object); err != nil {
		return fmt.Errorf("invalid tool arguments JSON: %w", err)
	}
	if object == nil {
		return errors.New("tool arguments must be a JSON object")
	}
	required, _ := def.Parameters["required"].([]any)
	properties, _ := def.Parameters["properties"].(map[string]any)
	for _, raw := range required {
		name, _ := raw.(string)
		if name != "" {
			if _, ok := object[name]; !ok {
				return fmt.Errorf("missing required argument %q", name)
			}
		}
	}
	for name, value := range object {
		property, _ := properties[name].(map[string]any)
		typeName, _ := property["type"].(string)
		if typeName != "" && !matchesJSONType(value, typeName) {
			return fmt.Errorf("argument %q must be %s", name, typeName)
		}
	}
	return nil
}

func matchesJSONType(value any, typeName string) bool {
	switch typeName {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		number, ok := value.(float64)
		return ok && number == float64(int64(number))
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	default:
		return false
	}
}

// LimitResult 截断过长工具输出：保留头尾并插入标记，避免撑爆模型上下文。
func LimitResult(result Result, max int) Result {
	if max <= 0 || len(result.Content) <= max {
		return result
	}
	marker := "...[truncated]..."
	if max <= len(marker) {
		result.Content = marker[:max]
	} else {
		head := max / 2
		tail := max - head - len(marker)
		result.Content = result.Content[:head] + marker + result.Content[len(result.Content)-tail:]
	}
	result.Truncated = true
	return result
}
