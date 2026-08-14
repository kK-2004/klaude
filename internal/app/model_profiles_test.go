package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kk-2004/klaude/internal/config"
	"github.com/kk-2004/klaude/internal/storage"
)

type memorySecrets struct{ values map[string]string }

func (m *memorySecrets) Save(account, value string) error {
	if m.values == nil {
		m.values = make(map[string]string)
	}
	m.values[account] = value
	return nil
}

func (m *memorySecrets) Load(account string) (string, error) { return m.values[account], nil }

func TestModelProfilePersistsMetadataAndCredentialReference(t *testing.T) {
	dirs := storage.NewDataDirs(filepath.Join(t.TempDir(), "profile"))
	service := NewServiceWithDataDirs(slog.Default(), dirs)
	service.Startup(context.Background())
	defer service.Shutdown(context.Background())
	secrets := &memorySecrets{}
	service.secrets = secrets

	catalog, err := service.SaveModelProfile(context.Background(), ModelProfileInput{
		ID: "custom-openai", Name: "Custom OpenAI", ProviderSpec: "openai", APIMode: "responses",
		BaseURL: "https://example.com/v1", Model: "example-model", APIKey: "plaintext-secret",
		ContextWindow: 200_000, MaxOutputTokens: 12_000, Temperature: 0.3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if catalog.ActiveID != "custom-openai" || len(catalog.Profiles) != 1 || !catalog.Profiles[0].HasAPIKey {
		t.Fatalf("catalog = %+v", catalog)
	}
	if secrets.values["model-custom-openai"] != "plaintext-secret" {
		t.Fatalf("secret was not saved under expected account: %+v", secrets.values)
	}
	if service.config.Config.Provider.Protocol != "openai" || service.config.Config.Provider.APIMode != "responses" || service.config.Config.Provider.CredentialKey != "model-custom-openai" || service.config.Config.Provider.CredentialEnv != "" {
		t.Fatalf("provider = %+v", service.config.Config.Provider)
	}
	configData, err := os.ReadFile(config.UserConfigPath(dirs.Base))
	if err != nil {
		t.Fatal(err)
	}
	setting, err := service.db.GetSetting(context.Background(), modelCatalogSettingKey)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configData), "plaintext-secret") || strings.Contains(setting.ValueJSON, "plaintext-secret") {
		t.Fatal("plaintext API key leaked into persistent metadata")
	}
	_, err = service.SaveModelProfile(context.Background(), ModelProfileInput{
		ID: "custom-openai", Name: "Changed protocol", ProviderSpec: "anthropic", APIMode: "messages",
		BaseURL: "https://api.anthropic.com/v1", Model: "claude-test",
		ContextWindow: 200_000, MaxOutputTokens: 12_000, Temperature: 0.3,
	})
	if err == nil || !strings.Contains(err.Error(), "API key is required") {
		t.Fatalf("provider change should require a matching credential, err=%v", err)
	}
}
