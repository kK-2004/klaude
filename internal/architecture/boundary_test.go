package architecture

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimePackagesDoNotImportDesktopOrFrontendTypes(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, pkg := range []string{"agent", "context", "model", "tool", "permission", "approval", "sandbox", "executor"} {
		dir := filepath.Join(root, "internal", pkg)
		entries, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, filename := range entries {
			file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", filename, err)
			}
			for _, imported := range file.Imports {
				path := strings.Trim(imported.Path.Value, `"`)
				if strings.Contains(path, "wails") || strings.Contains(path, "/frontend") || strings.Contains(path, "react") {
					t.Errorf("%s imports forbidden UI dependency %q", filename, path)
				}
			}
		}
	}
}

func TestToolPackagesDoNotImportFrontend(t *testing.T) {
	root := filepath.Join("..", "..", "internal", "tool")
	entries, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, filename := range entries {
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", filename, err)
		}
		for _, imported := range file.Imports {
			path := strings.Trim(imported.Path.Value, `"`)
			if strings.Contains(path, "/frontend") || strings.Contains(path, "react") {
				t.Errorf("%s imports frontend dependency %q", filename, path)
			}
		}
	}
}
