package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/kk-2004/klaude/internal/tool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type Snapshot struct {
	Definition
	Status string     `json:"status"`
	Error  string     `json:"error,omitempty"`
	Tools  []ToolInfo `json:"tools,omitempty"`
}

type Manager struct {
	mu          sync.RWMutex
	logger      *slog.Logger
	definitions map[string]Definition
	connections map[string]*connection
	failures    map[string]string
}

type connection struct {
	session *mcp.ClientSession
	tools   []tool.Tool
	info    []ToolInfo
}

func NewManager(definitions []Definition, logger *slog.Logger) *Manager {
	items := make(map[string]Definition, len(definitions))
	for _, item := range definitions {
		normalized, err := NormalizeDefinition(item)
		if err == nil {
			items[normalized.ID] = normalized
		}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{logger: logger, definitions: items, connections: make(map[string]*connection), failures: make(map[string]string)}
}

func (m *Manager) Upsert(definition Definition) (Definition, error) {
	normalized, err := NormalizeDefinition(definition)
	if err != nil {
		return Definition{}, err
	}
	if err := m.Disconnect(normalized.ID); err != nil {
		return Definition{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.definitions[normalized.ID] = normalized
	delete(m.failures, normalized.ID)
	return normalized, nil
}

func (m *Manager) Remove(ctx context.Context, id string) error {
	if err := m.Disconnect(id); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.definitions, id)
	delete(m.failures, id)
	return nil
}

func (m *Manager) Connect(ctx context.Context, id string) (Snapshot, error) {
	m.mu.RLock()
	definition, ok := m.definitions[id]
	m.mu.RUnlock()
	if !ok {
		return Snapshot{}, errors.New("MCP server was not found")
	}
	if err := m.Disconnect(id); err != nil {
		return Snapshot{}, err
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "Klaude", Version: "0.1.0"}, nil)
	transport, err := clientTransport(definition)
	if err != nil {
		return m.setFailure(definition, err), err
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return m.setFailure(definition, err), err
	}
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		_ = session.Close()
		return m.setFailure(definition, err), err
	}

	items := make([]tool.Tool, 0, len(listed.Tools))
	info := make([]ToolInfo, 0, len(listed.Tools))
	for _, item := range listed.Tools {
		if item == nil || strings.TrimSpace(item.Name) == "" {
			continue
		}
		name := NamespacedToolName(definition, item.Name)
		parameters := map[string]any{"type": "object"}
		if encoded, encodeErr := json.Marshal(item.InputSchema); encodeErr == nil {
			var schema map[string]any
			if json.Unmarshal(encoded, &schema) == nil && len(schema) > 0 {
				parameters = schema
			}
		}
		items = append(items, &remoteTool{session: session, name: name, remoteName: item.Name, description: item.Description, parameters: parameters})
		info = append(info, ToolInfo{Name: item.Name, Description: item.Description})
	}

	m.mu.Lock()
	m.connections[id] = &connection{session: session, tools: items, info: info}
	delete(m.failures, id)
	m.mu.Unlock()
	return Snapshot{Definition: definition, Status: "connected", Tools: info}, nil
}

func (m *Manager) Disconnect(id string) error {
	m.mu.Lock()
	item := m.connections[id]
	delete(m.connections, id)
	delete(m.failures, id)
	m.mu.Unlock()
	if item == nil || item.session == nil {
		return nil
	}
	return item.session.Close()
}

func (m *Manager) Snapshots() []Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.definitions))
	for id := range m.definitions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]Snapshot, 0, len(ids))
	for _, id := range ids {
		definition := m.definitions[id]
		snapshot := Snapshot{Definition: definition, Status: "disconnected"}
		if failure := m.failures[id]; failure != "" {
			snapshot.Status = "error"
			snapshot.Error = failure
		}
		if item := m.connections[id]; item != nil {
			snapshot.Status = "connected"
			snapshot.Tools = append([]ToolInfo(nil), item.info...)
		}
		result = append(result, snapshot)
	}
	return result
}

func (m *Manager) Tools() []tool.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []tool.Tool
	for _, item := range m.connections {
		result = append(result, item.tools...)
	}
	return result
}

func (m *Manager) Close() error {
	m.mu.Lock()
	connections := make([]*connection, 0, len(m.connections))
	for id, item := range m.connections {
		connections = append(connections, item)
		delete(m.connections, id)
	}
	m.mu.Unlock()
	var first error
	for _, item := range connections {
		if err := item.session.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func NamespacedToolName(definition Definition, name string) string {
	server := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, definition.ID)
	return "mcp__" + server + "__" + name
}

func clientTransport(definition Definition) (mcp.Transport, error) {
	switch definition.Transport {
	case TransportStreamableHTTP:
		return &mcp.StreamableClientTransport{Endpoint: definition.URL}, nil
	case TransportStdio:
		command := exec.Command(definition.Command, definition.Args...)
		if len(definition.Env) > 0 {
			// Keep the minimum process environment needed to resolve common CLI
			// runtimes, then pass through only the explicitly referenced secrets.
			env := make([]string, 0, len(definition.Env)+3)
			for _, name := range []string{"PATH", "HOME", "TMPDIR"} {
				if value, ok := os.LookupEnv(name); ok {
					env = append(env, name+"="+value)
				}
			}
			for _, name := range definition.Env {
				if value, ok := os.LookupEnv(name); ok {
					env = append(env, name+"="+value)
				}
			}
			command.Env = env
		}
		return &mcp.CommandTransport{Command: command}, nil
	default:
		return nil, fmt.Errorf("unsupported MCP transport %q", definition.Transport)
	}
}

func (m *Manager) setFailure(definition Definition, err error) Snapshot {
	m.logger.Warn("MCP connection failed", "server", definition.Name, "error", err)
	m.mu.Lock()
	m.failures[definition.ID] = err.Error()
	m.mu.Unlock()
	return Snapshot{Definition: definition, Status: "error", Error: err.Error()}
}

type remoteTool struct {
	session     *mcp.ClientSession
	name        string
	remoteName  string
	description string
	parameters  map[string]any
}

func (t *remoteTool) Definition() tool.Definition {
	return tool.Definition{Name: t.name, Description: t.description, Parameters: t.parameters, Metadata: tool.Metadata{Concurrent: true, RequiresApproval: true}}
}

func (t *remoteTool) Execute(ctx context.Context, arguments json.RawMessage) (tool.Result, error) {
	var values map[string]any
	if len(arguments) == 0 || string(arguments) == "null" {
		values = map[string]any{}
	} else if err := json.Unmarshal(arguments, &values); err != nil {
		return tool.Result{Success: false, ErrorCode: "invalid_arguments"}, err
	}
	result, err := t.session.CallTool(ctx, &mcp.CallToolParams{Name: t.remoteName, Arguments: values})
	if err != nil {
		return tool.Result{Success: false, ErrorCode: "mcp_call_failed"}, err
	}
	parts := make([]string, 0, len(result.Content))
	for _, item := range result.Content {
		if text, ok := item.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
			continue
		}
		encoded, encodeErr := json.Marshal(item)
		if encodeErr == nil {
			parts = append(parts, string(encoded))
		}
	}
	return tool.Result{Content: strings.Join(parts, "\n"), Success: !result.IsError}, nil
}
