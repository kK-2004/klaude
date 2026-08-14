package mcp

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/kk-2004/klaude/internal/storage"
)

const (
	TransportStreamableHTTP = "streamable_http"
	TransportStdio          = "stdio"
)

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Definition is the persisted, non-secret description of an MCP server.
// Env contains names inherited from the parent process; values are never saved.
type Definition struct {
	ID        string   `json:"id" toml:"id"`
	Name      string   `json:"name" toml:"name"`
	Transport string   `json:"transport" toml:"transport"`
	URL       string   `json:"url,omitempty" toml:"url,omitempty"`
	Command   string   `json:"command,omitempty" toml:"command,omitempty"`
	Args      []string `json:"args,omitempty" toml:"args,omitempty"`
	Env       []string `json:"env,omitempty" toml:"env,omitempty"`
	Enabled   bool     `json:"enabled" toml:"enabled"`
}

func NormalizeDefinition(input Definition) (Definition, error) {
	result := input
	result.ID = strings.TrimSpace(result.ID)
	if result.ID == "" {
		result.ID = storage.NewID()
	}
	result.Name = strings.TrimSpace(result.Name)
	if result.Name == "" {
		return Definition{}, errors.New("MCP server name is required")
	}
	result.Transport = strings.TrimSpace(result.Transport)
	switch result.Transport {
	case TransportStreamableHTTP:
		result.URL = strings.TrimSpace(result.URL)
		parsed, err := url.Parse(result.URL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return Definition{}, errors.New("MCP Streamable HTTP URL must use http or https")
		}
		result.Command, result.Args, result.Env = "", nil, nil
	case TransportStdio:
		result.Command = strings.TrimSpace(result.Command)
		if result.Command == "" {
			return Definition{}, errors.New("MCP stdio command is required")
		}
		result.URL = ""
		result.Args = trimValues(result.Args)
		result.Env = trimValues(result.Env)
		for _, name := range result.Env {
			if !envNamePattern.MatchString(name) {
				return Definition{}, fmt.Errorf("MCP environment reference %q is invalid", name)
			}
		}
	default:
		return Definition{}, errors.New("MCP transport must be streamable_http or stdio")
	}
	return result, nil
}

func trimValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
