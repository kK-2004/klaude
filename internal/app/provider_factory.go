package app

import (
	"errors"

	"github.com/klaude/klaude/internal/config"
	"github.com/klaude/klaude/internal/model"
	"github.com/klaude/klaude/internal/model/anthropic"
	"github.com/klaude/klaude/internal/model/openai"
)

func (s *Service) newConfiguredProvider(cfg config.ProviderConfig) (model.Provider, error) {
	if cfg.APIKey == "" && cfg.CredentialKey != "" {
		credential, err := s.secrets.Load(cfg.CredentialKey)
		if err != nil {
			return nil, err
		}
		cfg.APIKey = credential
	}
	switch cfg.Protocol {
	case "", "openai":
		return openai.New(cfg)
	case "anthropic":
		return anthropic.New(cfg)
	default:
		return nil, errors.New("unsupported model provider protocol")
	}
}
