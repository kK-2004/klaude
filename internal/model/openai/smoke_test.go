package openai

import (
	"context"
	"os"
	"testing"

	"github.com/kk-2004/klaude/internal/config"
	"github.com/kk-2004/klaude/internal/model"
)

// Opt-in only: CI and local default test runs never contact a provider and
// never print or persist the credential value.
func TestOptInProviderSmoke(t *testing.T) {
	if os.Getenv("KLAUDE_REAL_PROVIDER_SMOKE") != "1" {
		t.Skip("set KLAUDE_REAL_PROVIDER_SMOKE=1 to run the opt-in provider smoke test")
	}
	cfg := config.Defaults().Provider
	provider, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := provider.Stream(context.Background(), model.Request{Model: cfg.Model, Messages: []model.Message{{Role: model.RoleUser, Content: "Reply with one word: ok"}}, MaxTokens: 8})
	if err != nil {
		t.Fatal(err)
	}
	for range stream {
		// Consume the bounded response without logging content or usage.
	}
}
