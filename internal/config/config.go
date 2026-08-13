package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DefaultModel string           `toml:"default_model"`
	UI           UIConfig         `toml:"ui"`
	Agent        AgentConfig      `toml:"agent"`
	Provider     ProviderConfig   `toml:"provider"`
	Permissions  PermissionConfig `toml:"permissions"`
}

type UIConfig struct {
	Theme string `toml:"theme"`
}

type AgentConfig struct {
	MaxTurns           int `toml:"max_turns"`
	ContextBudgetChars int `toml:"context_budget_chars"`
	ToolResultChars    int `toml:"tool_result_chars"`
	ShellTimeoutSec    int `toml:"shell_timeout_sec"`
}

type ProviderConfig struct {
	Name              string `toml:"name"`
	Endpoint          string `toml:"endpoint"`
	Model             string `toml:"model"`
	CredentialEnv     string `toml:"credential_env"`
	AllowHTTPForLocal bool   `toml:"allow_http_for_local"`
	SupportsTools     bool   `toml:"supports_tools"`
}

type PermissionConfig struct {
	Read       string            `toml:"read"`
	Write      string            `toml:"write"`
	Shell      string            `toml:"shell"`
	Network    string            `toml:"network"`
	ShellRules map[string]string `toml:"shell_rules"`
}

type InstructionSource struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type LoadResult struct {
	Config       Config
	Sources      []string
	Instructions []InstructionSource
	Warnings     []error
}

func Defaults() Config {
	return Config{
		DefaultModel: "openai:gpt-4o-mini",
		UI:           UIConfig{Theme: "system"},
		Agent:        AgentConfig{MaxTurns: 50, ContextBudgetChars: 120_000, ToolResultChars: 24_000, ShellTimeoutSec: 120},
		Provider:     ProviderConfig{Name: "openai-compatible", Endpoint: "https://api.openai.com/v1", Model: "gpt-4o-mini", CredentialEnv: "OPENAI_API_KEY", SupportsTools: true},
		Permissions:  PermissionConfig{Read: "allow", Write: "ask", Shell: "ask", Network: "ask", ShellRules: map[string]string{}},
	}
}

// Load 叠加载入：Defaults → 用户配置 → 项目 .klaude/config.toml，并收集 instructions。
// 项目配置不得覆盖权限；未知字段/非法值记入 Warnings 而非硬失败。
func Load(userPath, projectRoot string) LoadResult {
	result := LoadResult{Config: Defaults()}
	if userPath != "" {
		result.loadFile(userPath, false)
	}
	if projectRoot != "" {
		projectConfig := filepath.Join(projectRoot, ".klaude", "config.toml")
		result.loadFile(projectConfig, true)
		result.loadInstructions(projectRoot)
	}
	if result.Config.DefaultModel == "" {
		result.Config.DefaultModel = result.Config.Provider.Name + ":" + result.Config.Provider.Model
	}
	return result
}

func (r *LoadResult) loadFile(path string, project bool) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		r.Warnings = append(r.Warnings, fmt.Errorf("read %s: %w", path, err))
		return
	}
	var patch Config
	metadata, err := toml.Decode(string(content), &patch)
	if err != nil {
		r.Warnings = append(r.Warnings, fmt.Errorf("parse %s: %w", path, err))
		return
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		r.Warnings = append(r.Warnings, fmt.Errorf("unknown fields in %s: %s", path, joinKeys(undecoded)))
		return
	}
	// 安全边界：项目级 TOML 不能放宽/改写权限策略。
	if project && hasPermissionPatch(patch.Permissions) {
		r.Warnings = append(r.Warnings, fmt.Errorf("project config cannot override permissions: %s", path))
		patch.Permissions = PermissionConfig{}
	}
	if err := Validate(patch, project); err != nil {
		r.Warnings = append(r.Warnings, fmt.Errorf("invalid config %s: %w", path, err))
		return
	}
	merge(&r.Config, patch, project)
	r.Sources = append(r.Sources, path)
}

func hasPermissionPatch(cfg PermissionConfig) bool {
	return cfg.Read != "" || cfg.Write != "" || cfg.Shell != "" || cfg.Network != "" || len(cfg.ShellRules) > 0
}

func (r *LoadResult) loadInstructions(projectRoot string) {
	for _, path := range []string{filepath.Join(projectRoot, ".klaude", "instructions.md"), filepath.Join(projectRoot, "AGENTS.md")} {
		content, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			r.Warnings = append(r.Warnings, fmt.Errorf("read instructions %s: %w", path, err))
			continue
		}
		r.Instructions = append(r.Instructions, InstructionSource{Path: path, Content: string(content)})
	}
}

func Validate(cfg Config, project bool) error {
	if cfg.Agent.MaxTurns < 0 || cfg.Agent.MaxTurns > 1000 {
		return errors.New("agent.max_turns must be between 0 and 1000")
	}
	if cfg.Agent.ContextBudgetChars < 0 || cfg.Agent.ContextBudgetChars > 10_000_000 {
		return errors.New("agent.context_budget_chars is out of range")
	}
	if cfg.Agent.ToolResultChars < 0 || cfg.Agent.ToolResultChars > 1_000_000 {
		return errors.New("agent.tool_result_chars is out of range")
	}
	if cfg.Agent.ShellTimeoutSec < 0 || cfg.Agent.ShellTimeoutSec > 86_400 {
		return errors.New("agent.shell_timeout_sec is out of range")
	}
	if cfg.Provider.Endpoint != "" {
		if !strings.HasPrefix(cfg.Provider.Endpoint, "https://") && !(cfg.Provider.AllowHTTPForLocal && strings.HasPrefix(cfg.Provider.Endpoint, "http://localhost")) {
			return errors.New("provider.endpoint must use https, except explicit localhost development mode")
		}
	}
	if project && cfg.Provider.CredentialEnv != "" && strings.Contains(strings.ToLower(cfg.Provider.CredentialEnv), "key=") {
		return errors.New("project config cannot persist plaintext credentials")
	}
	return nil
}

// ResolveCredential 只从环境变量取密钥，配置里只存变量名，避免明文落盘。
func ResolveCredential(ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", errors.New("credential environment variable is empty")
	}
	value, ok := os.LookupEnv(ref)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("credential environment variable %q is missing", ref)
	}
	return value, nil
}

func merge(dst *Config, patch Config, project bool) {
	if patch.DefaultModel != "" {
		dst.DefaultModel = patch.DefaultModel
	}
	if patch.UI.Theme != "" {
		dst.UI.Theme = patch.UI.Theme
	}
	if patch.Agent.MaxTurns != 0 {
		dst.Agent.MaxTurns = patch.Agent.MaxTurns
	}
	if patch.Agent.ContextBudgetChars != 0 {
		dst.Agent.ContextBudgetChars = patch.Agent.ContextBudgetChars
	}
	if patch.Agent.ToolResultChars != 0 {
		dst.Agent.ToolResultChars = patch.Agent.ToolResultChars
	}
	if patch.Agent.ShellTimeoutSec != 0 {
		dst.Agent.ShellTimeoutSec = patch.Agent.ShellTimeoutSec
	}
	if patch.Provider.Name != "" {
		dst.Provider.Name = patch.Provider.Name
	}
	if patch.Provider.Endpoint != "" {
		dst.Provider.Endpoint = patch.Provider.Endpoint
	}
	if patch.Provider.Model != "" {
		dst.Provider.Model = patch.Provider.Model
	}
	if patch.Provider.CredentialEnv != "" {
		dst.Provider.CredentialEnv = patch.Provider.CredentialEnv
	}
	if !project {
		dst.Provider.AllowHTTPForLocal = patch.Provider.AllowHTTPForLocal
		dst.Provider.SupportsTools = patch.Provider.SupportsTools
	}
	if !project && patch.Permissions.ShellRules != nil {
		dst.Permissions.ShellRules = patch.Permissions.ShellRules
	}
	if !project {
		if patch.Permissions.Read != "" {
			dst.Permissions.Read = patch.Permissions.Read
		}
		if patch.Permissions.Write != "" {
			dst.Permissions.Write = patch.Permissions.Write
		}
		if patch.Permissions.Shell != "" {
			dst.Permissions.Shell = patch.Permissions.Shell
		}
		if patch.Permissions.Network != "" {
			dst.Permissions.Network = patch.Permissions.Network
		}
	}
}

func joinKeys(keys []toml.Key) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key.String())
	}
	return strings.Join(parts, ", ")
}

func (r LoadResult) LoadedAt() time.Time { return time.Now().UTC() }
