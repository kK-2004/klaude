package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kk-2004/klaude/internal/config"
	"github.com/kk-2004/klaude/internal/model"
	"github.com/kk-2004/klaude/internal/storage"
)

const modelCatalogSettingKey = "model_profiles"

type ModelProfile struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	ProviderSpec    string  `json:"providerSpec"`
	APIMode         string  `json:"apiMode"`
	BaseURL         string  `json:"baseUrl"`
	Model           string  `json:"model"`
	ContextWindow   int     `json:"contextWindow"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
	Temperature     float64 `json:"temperature"`
	HasAPIKey       bool    `json:"hasApiKey"`
}

type ModelProfileInput struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	ProviderSpec    string  `json:"providerSpec"`
	APIMode         string  `json:"apiMode"`
	BaseURL         string  `json:"baseUrl"`
	Model           string  `json:"model"`
	APIKey          string  `json:"apiKey"`
	ContextWindow   int     `json:"contextWindow"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
	Temperature     float64 `json:"temperature"`
}

type ModelCatalog struct {
	ActiveID string         `json:"activeId"`
	Profiles []ModelProfile `json:"profiles"`
}

type ModelConnectionResult struct {
	Success   bool   `json:"success"`
	LatencyMS int64  `json:"latencyMs"`
	Message   string `json:"message"`
}

type storedModelProfile struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	ProviderSpec    string  `json:"providerSpec"`
	APIMode         string  `json:"apiMode"`
	BaseURL         string  `json:"baseUrl"`
	Model           string  `json:"model"`
	CredentialEnv   string  `json:"credentialEnv,omitempty"`
	CredentialKey   string  `json:"credentialKey,omitempty"`
	ContextWindow   int     `json:"contextWindow"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
	Temperature     float64 `json:"temperature"`
}

type storedModelCatalog struct {
	ActiveID string               `json:"activeId"`
	Profiles []storedModelProfile `json:"profiles"`
}

func (s *Service) ModelProfiles(ctx context.Context) (ModelCatalog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	catalog, err := s.loadModelCatalog(ctx, s.config.Config)
	if err != nil {
		return ModelCatalog{}, err
	}
	return publicModelCatalog(catalog), nil
}

func (s *Service) SaveModelProfile(ctx context.Context, input ModelProfileInput) (ModelCatalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile, err := normalizeModelProfile(input)
	if err != nil {
		return ModelCatalog{}, err
	}
	catalog, err := s.loadModelCatalog(ctx, s.config.Config)
	if err != nil {
		return ModelCatalog{}, err
	}
	index := -1
	for i := range catalog.Profiles {
		if catalog.Profiles[i].ID == profile.ID {
			index = i
			if catalog.Profiles[i].ProviderSpec == profile.ProviderSpec {
				profile.CredentialEnv = catalog.Profiles[i].CredentialEnv
				profile.CredentialKey = catalog.Profiles[i].CredentialKey
			}
			break
		}
	}
	if strings.TrimSpace(input.APIKey) != "" {
		account := "model-" + profile.ID
		if err := s.secrets.Save(account, strings.TrimSpace(input.APIKey)); err != nil {
			return ModelCatalog{}, fmt.Errorf("save API key to system keychain: %w", err)
		}
		profile.CredentialKey, profile.CredentialEnv = account, ""
	}
	if profile.CredentialKey == "" && profile.CredentialEnv == "" {
		return ModelCatalog{}, errors.New("API key is required for a new model profile")
	}
	if index >= 0 {
		catalog.Profiles[index] = profile
	} else {
		catalog.Profiles = append(catalog.Profiles, profile)
	}
	catalog.ActiveID = profile.ID
	if err := s.persistModelCatalog(ctx, &catalog, profile); err != nil {
		return ModelCatalog{}, err
	}
	return publicModelCatalog(catalog), nil
}

func (s *Service) SelectModelProfile(ctx context.Context, profileID string) (ModelCatalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	catalog, err := s.loadModelCatalog(ctx, s.config.Config)
	if err != nil {
		return ModelCatalog{}, err
	}
	profile, ok := findStoredProfile(catalog, profileID)
	if !ok {
		return ModelCatalog{}, errors.New("model profile was not found")
	}
	catalog.ActiveID = profile.ID
	if err := s.persistModelCatalog(ctx, &catalog, profile); err != nil {
		return ModelCatalog{}, err
	}
	return publicModelCatalog(catalog), nil
}

func (s *Service) TestModelConnection(ctx context.Context, input ModelProfileInput) (ModelConnectionResult, error) {
	profile, err := normalizeModelProfile(input)
	if err != nil {
		return ModelConnectionResult{}, err
	}
	s.mu.RLock()
	catalog, catalogErr := s.loadModelCatalog(ctx, s.config.Config)
	s.mu.RUnlock()
	if catalogErr == nil {
		if stored, ok := findStoredProfile(catalog, profile.ID); ok {
			profile.CredentialEnv, profile.CredentialKey = stored.CredentialEnv, stored.CredentialKey
		}
	}
	providerConfig := configFromStoredProfile(profile)
	if strings.TrimSpace(input.APIKey) != "" {
		providerConfig.APIKey = strings.TrimSpace(input.APIKey)
	} else if profile.CredentialKey != "" {
		providerConfig.APIKey, err = s.secrets.Load(profile.CredentialKey)
		if err != nil {
			return ModelConnectionResult{}, fmt.Errorf("load API key from system keychain: %w", err)
		}
	}
	provider, err := s.newConfiguredProvider(providerConfig)
	if err != nil {
		return ModelConnectionResult{}, err
	}
	testContext, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	started := time.Now()
	stream, err := provider.Stream(testContext, model.Request{Model: profile.Model, Messages: []model.Message{{Role: model.RoleUser, Content: "Reply with OK only."}}, MaxTokens: 8})
	if err != nil {
		return ModelConnectionResult{LatencyMS: time.Since(started).Milliseconds(), Message: err.Error()}, nil
	}
	var responseText string
	for item := range stream {
		if item.Type == model.TextDelta {
			responseText += item.Text
		}
		if item.Type == model.ModelCompleted {
			return ModelConnectionResult{Success: true, LatencyMS: time.Since(started).Milliseconds(), Message: strings.TrimSpace(responseText)}, nil
		}
	}
	if err := testContext.Err(); err != nil {
		return ModelConnectionResult{LatencyMS: time.Since(started).Milliseconds(), Message: err.Error()}, nil
	}
	return ModelConnectionResult{LatencyMS: time.Since(started).Milliseconds(), Message: "model stream ended before completion"}, nil
}

func (s *Service) loadModelCatalog(ctx context.Context, cfg config.Config) (storedModelCatalog, error) {
	if s.db != nil {
		setting, err := s.db.GetSetting(ctx, modelCatalogSettingKey)
		if err == nil {
			var catalog storedModelCatalog
			if err := json.Unmarshal([]byte(setting.ValueJSON), &catalog); err != nil {
				return storedModelCatalog{}, err
			}
			if len(catalog.Profiles) > 0 {
				return catalog, nil
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return storedModelCatalog{}, err
		}
	}
	if strings.TrimSpace(cfg.Provider.Model) == "" {
		return storedModelCatalog{Profiles: []storedModelProfile{}}, nil
	}
	profile := storedProfileFromConfig(cfg.Provider)
	return storedModelCatalog{ActiveID: profile.ID, Profiles: []storedModelProfile{profile}}, nil
}

func (s *Service) persistModelCatalog(ctx context.Context, catalog *storedModelCatalog, active storedModelProfile) error {
	cfg := s.config.Config
	cfg.Provider = configFromStoredProfile(active)
	cfg.DefaultModel = cfg.Provider.Name + ":" + cfg.Provider.Model
	if err := config.Validate(cfg, false); err != nil {
		return err
	}
	if err := config.Save(config.UserConfigPath(s.data.Base), cfg); err != nil {
		return err
	}
	encoded, err := json.Marshal(catalog)
	if err != nil {
		return err
	}
	if s.db == nil {
		return errors.New("model profile storage is unavailable")
	}
	if err := s.db.SetSetting(ctx, storage.Setting{Key: modelCatalogSettingKey, ValueJSON: string(encoded)}); err != nil {
		return err
	}
	s.config.Config = cfg
	if s.composition != nil {
		s.composition.Config.Config = cfg
	}
	return nil
}

func normalizeModelProfile(input ModelProfileInput) (storedModelProfile, error) {
	profile := storedModelProfile{
		ID: strings.TrimSpace(input.ID), Name: strings.TrimSpace(input.Name), ProviderSpec: strings.TrimSpace(input.ProviderSpec),
		APIMode: strings.TrimSpace(input.APIMode), BaseURL: strings.TrimRight(strings.TrimSpace(input.BaseURL), "/"), Model: strings.TrimSpace(input.Model),
		ContextWindow: input.ContextWindow, MaxOutputTokens: input.MaxOutputTokens, Temperature: input.Temperature,
	}
	if profile.ID == "" {
		profile.ID = storage.NewID()
	}
	if profile.Name == "" {
		profile.Name = profile.Model
	}
	if profile.ProviderSpec == "anthropic" {
		profile.APIMode = "messages"
	}
	if profile.ProviderSpec != "openai" && profile.ProviderSpec != "anthropic" {
		return storedModelProfile{}, errors.New("provider specification must be openai or anthropic")
	}
	if profile.APIMode != "chat_completions" && profile.APIMode != "responses" && profile.APIMode != "messages" {
		return storedModelProfile{}, errors.New("API mode is invalid")
	}
	if profile.ProviderSpec == "openai" && profile.APIMode == "messages" {
		return storedModelProfile{}, errors.New("OpenAI specification requires Chat Completions or Responses API mode")
	}
	if profile.BaseURL == "" || profile.Model == "" {
		return storedModelProfile{}, errors.New("Base URL and model ID are required")
	}
	if profile.ContextWindow <= 0 || profile.MaxOutputTokens <= 0 {
		return storedModelProfile{}, errors.New("context window and maximum output tokens must be greater than zero")
	}
	if profile.Temperature < 0 || profile.Temperature > 2 {
		return storedModelProfile{}, errors.New("temperature must be between 0 and 2")
	}
	if err := config.Validate(config.Config{Provider: configFromStoredProfile(profile)}, false); err != nil {
		return storedModelProfile{}, err
	}
	return profile, nil
}

func storedProfileFromConfig(provider config.ProviderConfig) storedModelProfile {
	protocol := provider.Protocol
	if protocol == "" {
		protocol = "openai"
	}
	mode := provider.APIMode
	if mode == "" {
		mode = "chat_completions"
	}
	return storedModelProfile{ID: "default", Name: provider.Model, ProviderSpec: protocol, APIMode: mode, BaseURL: provider.Endpoint, Model: provider.Model, CredentialEnv: provider.CredentialEnv, CredentialKey: provider.CredentialKey, ContextWindow: positiveOr(provider.ContextWindow, 128_000), MaxOutputTokens: positiveOr(provider.MaxOutputTokens, 16_384), Temperature: provider.Temperature}
}

func configFromStoredProfile(profile storedModelProfile) config.ProviderConfig {
	return config.ProviderConfig{
		Name: profile.ProviderSpec + "-compatible", Protocol: profile.ProviderSpec, APIMode: profile.APIMode,
		Endpoint: profile.BaseURL, Model: profile.Model, CredentialEnv: profile.CredentialEnv, CredentialKey: profile.CredentialKey,
		ContextWindow: profile.ContextWindow, MaxOutputTokens: profile.MaxOutputTokens, Temperature: profile.Temperature,
		AllowHTTPForLocal: strings.HasPrefix(profile.BaseURL, "http://localhost") || strings.HasPrefix(profile.BaseURL, "http://127.0.0.1"), SupportsTools: true,
	}
}

func publicModelCatalog(catalog storedModelCatalog) ModelCatalog {
	result := ModelCatalog{ActiveID: catalog.ActiveID, Profiles: make([]ModelProfile, 0, len(catalog.Profiles))}
	for _, profile := range catalog.Profiles {
		result.Profiles = append(result.Profiles, ModelProfile{ID: profile.ID, Name: profile.Name, ProviderSpec: profile.ProviderSpec, APIMode: profile.APIMode, BaseURL: profile.BaseURL, Model: profile.Model, ContextWindow: profile.ContextWindow, MaxOutputTokens: profile.MaxOutputTokens, Temperature: profile.Temperature, HasAPIKey: profile.CredentialKey != "" || profile.CredentialEnv != ""})
	}
	return result
}

func findStoredProfile(catalog storedModelCatalog, id string) (storedModelProfile, bool) {
	for _, profile := range catalog.Profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return storedModelProfile{}, false
}

func positiveOr(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
