package sandbox

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ensureSessionTempGeneric(sessionID, workspace string) (string, error) {
	base := filepath.Join(os.TempDir(), "klaude-sbx")
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", err
	}
	key := sessionID
	if strings.TrimSpace(key) == "" {
		key = workspace
	}
	sum := sha1.Sum([]byte(key))
	dir := filepath.Join(base, hex.EncodeToString(sum[:8]))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	_ = os.Chtimes(dir, time.Now(), time.Now())
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return dir, nil
	}
	return resolved, nil
}
