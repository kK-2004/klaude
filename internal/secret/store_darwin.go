//go:build darwin

package secret

import (
	"errors"
	"os/exec"
	"strings"
)

const keychainService = "com.klaude.model-credentials"

type keychainStore struct{}

func NewStore() Store { return keychainStore{} }

func (keychainStore) Save(account, value string) error {
	if strings.TrimSpace(account) == "" || strings.TrimSpace(value) == "" {
		return errors.New("credential account and value are required")
	}
	return exec.Command("/usr/bin/security", "add-generic-password", "-U", "-s", keychainService, "-a", account, "-w", value).Run()
}

func (keychainStore) Load(account string) (string, error) {
	if strings.TrimSpace(account) == "" {
		return "", errors.New("credential account is required")
	}
	output, err := exec.Command("/usr/bin/security", "find-generic-password", "-s", keychainService, "-a", account, "-w").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
