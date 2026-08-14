package app

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/kk-2004/klaude/internal/storage"
)

func TestBuildCompositionInitializesCorePorts(t *testing.T) {
	composition, err := Build(context.Background(), storage.NewDataDirs(filepath.Join(t.TempDir(), "profile")), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer composition.Close()
	if composition.DB == nil || composition.Events == nil || composition.Approvals == nil || composition.Projects == nil || composition.Sessions == nil || composition.Tools == nil {
		t.Fatal("composition is incomplete")
	}
}
