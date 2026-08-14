package app

import (
	"context"
	"errors"
	"strings"

	"github.com/kk-2004/klaude/internal/config"
	"github.com/kk-2004/klaude/internal/mcp"
)

type MCPServerInput struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	URL       string   `json:"url"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Env       []string `json:"env"`
	Enabled   bool     `json:"enabled"`
}

type MCPToolDTO struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type MCPServerDTO struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Transport string       `json:"transport"`
	URL       string       `json:"url,omitempty"`
	Command   string       `json:"command,omitempty"`
	Args      []string     `json:"args,omitempty"`
	Env       []string     `json:"env,omitempty"`
	Enabled   bool         `json:"enabled"`
	Status    string       `json:"status"`
	Error     string       `json:"error,omitempty"`
	Tools     []MCPToolDTO `json:"tools,omitempty"`
}

func (s *Service) MCPServers() []MCPServerDTO {
	s.mu.RLock()
	composition := s.composition
	s.mu.RUnlock()
	if composition == nil || composition.MCP == nil {
		return []MCPServerDTO{}
	}
	return mapMCPSnapshots(composition.MCP.Snapshots())
}

func (s *Service) SaveMCPServer(_ context.Context, input MCPServerInput) ([]MCPServerDTO, error) {
	definition, err := mcp.NormalizeDefinition(mcp.Definition{ID: input.ID, Name: input.Name, Transport: input.Transport, URL: input.URL, Command: input.Command, Args: input.Args, Env: input.Env, Enabled: input.Enabled})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.composition == nil || s.data.Base == "" {
		return nil, errors.New("MCP storage is unavailable")
	}
	cfg := s.config.Config
	replaced := false
	for index := range cfg.MCPServers {
		if cfg.MCPServers[index].ID == definition.ID {
			cfg.MCPServers[index] = definition
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.MCPServers = append(cfg.MCPServers, definition)
	}
	if err := config.Save(config.UserConfigPath(s.data.Base), cfg); err != nil {
		return nil, err
	}
	if _, err := s.composition.MCP.Upsert(definition); err != nil {
		return nil, err
	}
	s.config.Config = cfg
	s.composition.Config.Config = cfg
	return mapMCPSnapshots(s.composition.MCP.Snapshots()), nil
}

func (s *Service) DeleteMCPServer(ctx context.Context, id string) ([]MCPServerDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.composition == nil || s.data.Base == "" {
		return nil, errors.New("MCP storage is unavailable")
	}
	if err := s.composition.MCP.Remove(ctx, strings.TrimSpace(id)); err != nil {
		return nil, err
	}
	cfg := s.config.Config
	filtered := cfg.MCPServers[:0]
	for _, item := range cfg.MCPServers {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	cfg.MCPServers = filtered
	if err := config.Save(config.UserConfigPath(s.data.Base), cfg); err != nil {
		return nil, err
	}
	s.config.Config = cfg
	s.composition.Config.Config = cfg
	return mapMCPSnapshots(s.composition.MCP.Snapshots()), nil
}

func (s *Service) ConnectMCPServer(ctx context.Context, id string) ([]MCPServerDTO, error) {
	s.mu.RLock()
	composition := s.composition
	s.mu.RUnlock()
	if composition == nil || composition.MCP == nil {
		return nil, errors.New("MCP manager is unavailable")
	}
	if _, err := composition.MCP.Connect(ctx, strings.TrimSpace(id)); err != nil {
		return mapMCPSnapshots(composition.MCP.Snapshots()), err
	}
	return mapMCPSnapshots(composition.MCP.Snapshots()), nil
}

func (s *Service) DisconnectMCPServer(id string) ([]MCPServerDTO, error) {
	s.mu.RLock()
	composition := s.composition
	s.mu.RUnlock()
	if composition == nil || composition.MCP == nil {
		return nil, errors.New("MCP manager is unavailable")
	}
	if err := composition.MCP.Disconnect(strings.TrimSpace(id)); err != nil {
		return mapMCPSnapshots(composition.MCP.Snapshots()), err
	}
	return mapMCPSnapshots(composition.MCP.Snapshots()), nil
}

func mapMCPSnapshots(snapshots []mcp.Snapshot) []MCPServerDTO {
	result := make([]MCPServerDTO, 0, len(snapshots))
	for _, item := range snapshots {
		tools := make([]MCPToolDTO, 0, len(item.Tools))
		for _, discovered := range item.Tools {
			tools = append(tools, MCPToolDTO{Name: discovered.Name, Description: discovered.Description})
		}
		result = append(result, MCPServerDTO{ID: item.ID, Name: item.Name, Transport: item.Transport, URL: item.URL, Command: item.Command, Args: item.Args, Env: item.Env, Enabled: item.Enabled, Status: item.Status, Error: item.Error, Tools: tools})
	}
	return result
}
