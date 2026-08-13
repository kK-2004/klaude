package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/klaude/klaude/internal/filesystem"
)

// BuiltinContext 是只读内置工具共享的工作区与输出上限。
type BuiltinContext struct {
	Workspace filesystem.Service
	MaxOutput int
	RGPath    string
}

// RegisterReadOnly 注册 dig/list/grep/glob 等只读探索工具。
func RegisterReadOnly(registry *Registry, ctx BuiltinContext) error {
	for _, item := range []Tool{ReadFileTool{ctx}, ListDirectoryTool{ctx}, GrepTool{ctx}, GlobTool{ctx}} {
		if err := registry.Register(item); err != nil {
			return err
		}
	}
	return nil
}

type ReadFileTool struct{ Ctx BuiltinContext }

func (t ReadFileTool) Definition() Definition {
	return Definition{Name: "read_file", Description: "Read a text file inside the workspace", Parameters: objectSchema(map[string]any{"path": map[string]any{"type": "string"}, "start_line": map[string]any{"type": "integer"}, "end_line": map[string]any{"type": "integer"}}, "path"), Metadata: Metadata{ReadOnly: true, Concurrent: true}}
}
func (t ReadFileTool) Execute(ctx context.Context, arguments json.RawMessage) (Result, error) {
	var args struct {
		Path      string `json:"path"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return Result{ErrorCode: "invalid_arguments"}, err
	}
	content, err := t.Ctx.Workspace.ReadFile(ctx, args.Path)
	if err != nil {
		return Result{ErrorCode: "read_failed"}, err
	}
	if bytesContainZero(content) {
		return Result{ErrorCode: "binary_file"}, errors.New("file appears to be binary")
	}
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	start, end := 1, len(lines)
	if args.StartLine > 0 {
		start = args.StartLine
	}
	if args.EndLine > 0 {
		end = args.EndLine
	}
	if start < 1 || start > len(lines)+1 || end < start {
		return Result{ErrorCode: "invalid_line_range"}, errors.New("invalid line range")
	}
	if end > len(lines) {
		end = len(lines)
	}
	var builder strings.Builder
	for index := start; index <= end; index++ {
		fmt.Fprintf(&builder, "%d\t%s\n", index, lines[index-1])
	}
	return LimitResult(Result{Content: builder.String(), Success: true, Metadata: map[string]any{"startLine": start, "endLine": end}}, t.Ctx.MaxOutput), nil
}

type ListDirectoryTool struct{ Ctx BuiltinContext }

func (t ListDirectoryTool) Definition() Definition {
	return Definition{Name: "list_directory", Description: "List a directory inside the workspace", Parameters: objectSchema(map[string]any{"path": map[string]any{"type": "string"}}, "path"), Metadata: Metadata{ReadOnly: true, Concurrent: true}}
}
func (t ListDirectoryTool) Execute(ctx context.Context, arguments json.RawMessage) (Result, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return Result{ErrorCode: "invalid_arguments"}, err
	}
	entries, err := t.Ctx.Workspace.ListDirectory(ctx, args.Path)
	if err != nil {
		return Result{ErrorCode: "list_failed"}, err
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return Result{ErrorCode: "encode_failed"}, err
	}
	return LimitResult(Result{Content: string(data), Success: true}, t.Ctx.MaxOutput), nil
}

type GrepTool struct{ Ctx BuiltinContext }

func (t GrepTool) Definition() Definition {
	return Definition{Name: "grep", Description: "Search text with ripgrep inside the workspace", Parameters: objectSchema(map[string]any{"pattern": map[string]any{"type": "string"}, "path": map[string]any{"type": "string"}}, "pattern"), Metadata: Metadata{ReadOnly: true, Concurrent: true}}
}
func (t GrepTool) Execute(ctx context.Context, arguments json.RawMessage) (Result, error) {
	var args struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return Result{ErrorCode: "invalid_arguments"}, err
	}
	return runRG(ctx, t.Ctx, args.Pattern, args.Path, "--line-number", "--color", "never")
}

type GlobTool struct{ Ctx BuiltinContext }

func (t GlobTool) Definition() Definition {
	return Definition{Name: "glob", Description: "List matching files with ripgrep inside the workspace", Parameters: objectSchema(map[string]any{"pattern": map[string]any{"type": "string"}}, "pattern"), Metadata: Metadata{ReadOnly: true, Concurrent: true}}
}
func (t GlobTool) Execute(ctx context.Context, arguments json.RawMessage) (Result, error) {
	var args struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return Result{ErrorCode: "invalid_arguments"}, err
	}
	return runRG(ctx, t.Ctx, args.Pattern, "", "--files", "-g")
}

// runRG 在工作区根目录调用 ripgrep；exit code 1（无匹配）视为成功空结果。
func runRG(ctx context.Context, cfg BuiltinContext, pattern string, path string, flags ...string) (Result, error) {
	rg := cfg.RGPath
	if rg == "" {
		rg, _ = exec.LookPath("rg")
	}
	if rg == "" {
		return Result{ErrorCode: "dependency_unavailable"}, errors.New("ripgrep is not available")
	}
	if strings.TrimSpace(pattern) == "" {
		return Result{ErrorCode: "invalid_arguments"}, errors.New("pattern is empty")
	}
	args := append([]string{}, flags...)
	args = append(args, pattern)
	if path == "" {
		path = "."
	}
	args = append(args, path)
	command := exec.CommandContext(ctx, rg, args...)
	command.Dir = cfg.Workspace.Root
	output, err := command.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return Result{Content: "", Success: true}, nil
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return Result{ErrorCode: "cancelled"}, ctx.Err()
		}
		return Result{ErrorCode: "search_failed"}, err
	}
	return LimitResult(Result{Content: string(output), Success: true, Metadata: map[string]any{"command": "rg"}}, cfg.MaxOutput), nil
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	req := make([]any, 0, len(required))
	for _, name := range required {
		req = append(req, name)
	}
	return map[string]any{"type": "object", "properties": properties, "required": req}
}
func bytesContainZero(data []byte) bool {
	for _, value := range data {
		if value == 0 {
			return true
		}
	}
	return false
}
