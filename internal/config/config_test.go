package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kk-2004/klaude/internal/mcp"
)

func TestLayeredConfigAndInstructions(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(project, ".klaude"), 0o700); err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(root, "config.toml")
	if err := os.WriteFile(user, []byte("default_model = \"openai:user\"\n[provider]\nmodel = \"user-model\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".klaude", "config.toml"), []byte("[provider]\nmodel = \"project-model\"\n[permissions]\nwrite = \"allow\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".klaude", "instructions.md"), []byte("Project rules"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Load(user, project)
	if result.Config.Provider.Model != "project-model" {
		t.Fatalf("model = %q", result.Config.Provider.Model)
	}
	if result.Config.Permissions.Write != "ask" {
		t.Fatalf("project config weakened permissions: %q", result.Config.Permissions.Write)
	}
	if len(result.Instructions) != 1 || result.Instructions[0].Content != "Project rules" {
		t.Fatalf("instructions = %+v", result.Instructions)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0].Error(), "cannot override permissions") {
		t.Fatalf("warnings = %+v", result.Warnings)
	}
}

func TestConfigRejectsInsecureEndpointAndPlaintextCredential(t *testing.T) {
	cfg := Defaults()
	cfg.Provider.Endpoint = "http://example.com"
	if err := Validate(cfg, false); err == nil {
		t.Fatal("expected insecure endpoint error")
	}
	if err := Validate(cfg, true); err == nil {
		t.Fatal("expected project endpoint error")
	}
	if err := os.Setenv("KLAUDE_TEST_SECRET", "secret"); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("KLAUDE_TEST_SECRET")
	got, err := ResolveCredential("KLAUDE_TEST_SECRET")
	if err != nil || got != "secret" {
		t.Fatalf("credential = %q, err=%v", got, err)
	}
}

func TestAgentScheduleFlagsDefaultOff(t *testing.T) {
	cfg := Defaults()
	if cfg.Agent.ParallelTools || cfg.Agent.LLMSchedule {
		t.Fatalf("expected parallel flags off by default: %+v", cfg.Agent)
	}
}

func TestDefaultsDoNotSelectAModel(t *testing.T) {
	cfg := Defaults()
	if cfg.DefaultModel != "" {
		t.Fatalf("default model = %q, want empty", cfg.DefaultModel)
	}
	if cfg.Provider.Model != "" {
		t.Fatalf("provider model = %q, want empty", cfg.Provider.Model)
	}
}

func TestSaveAndReloadParallelFlags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Defaults()
	cfg.Agent.ParallelTools = true
	cfg.Agent.LLMSchedule = true
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded := Load(path, "")
	if !loaded.Config.Agent.ParallelTools || !loaded.Config.Agent.LLMSchedule {
		t.Fatalf("loaded = %+v", loaded.Config.Agent)
	}
}

func TestSaveNeverPersistsPlaintextAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Defaults()
	cfg.Provider.APIKey = "super-secret-value"
	cfg.Provider.CredentialKey = "model-test"
	cfg.Provider.CredentialEnv = ""
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "super-secret-value") {
		t.Fatal("plaintext API key was written to config")
	}
	loaded := Load(path, "")
	if loaded.Config.Provider.CredentialKey != "model-test" || loaded.Config.Provider.CredentialEnv != "" {
		t.Fatalf("credential reference = %+v", loaded.Config.Provider)
	}
}

func TestSaveAndReloadMCPDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Defaults()
	cfg.MCPServers = []mcp.Definition{{ID: "files", Name: "Files", Transport: mcp.TransportStdio, Command: "npx", Args: []string{"-y", "server"}, Env: []string{"API_KEY"}, Enabled: true}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded := Load(path, "")
	if len(loaded.Config.MCPServers) != 1 || loaded.Config.MCPServers[0].Command != "npx" || loaded.Config.MCPServers[0].Args[1] != "server" || !loaded.Config.MCPServers[0].Enabled {
		t.Fatalf("loaded MCP servers = %+v, warnings=%v", loaded.Config.MCPServers, loaded.Warnings)
	}
}
