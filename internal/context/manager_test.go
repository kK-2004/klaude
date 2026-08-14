package context

import (
	"context"
	"strings"
	"testing"

	"github.com/kk-2004/klaude/internal/model"
)

func TestBuildPreservesInstructionsAndTruncatesToolResult(t *testing.T) {
	manager := Manager{SystemInstructions: "system", ProjectInstructions: "project", BudgetChars: 1000, OutputReserveChars: 100, ToolResultChars: 20}
	request, err := manager.Build(context.Background(), []model.Message{{Role: model.RoleUser, Content: "hello"}, {Role: model.RoleTool, Content: strings.Repeat("x", 100)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(request.Messages) != 4 || !strings.Contains(request.Messages[3].Content, "truncated") {
		t.Fatalf("messages = %+v", request.Messages)
	}
}

func TestBuildFailsWhenRequiredContentCannotFit(t *testing.T) {
	manager := Manager{BudgetChars: 10, OutputReserveChars: 9}
	if _, err := manager.Build(context.Background(), []model.Message{{Role: model.RoleUser, Content: "too large"}}); err == nil {
		t.Fatal("expected context budget error")
	}
}

func BenchmarkBuildBoundedContext(b *testing.B) {
	manager := Manager{SystemInstructions: "system", ProjectInstructions: "project", BudgetChars: 120_000, OutputReserveChars: 12_000, ToolResultChars: 24_000}
	messages := []model.Message{{Role: model.RoleUser, Content: strings.Repeat("inspect this repository ", 300)}, {Role: model.RoleTool, Content: strings.Repeat("tool output ", 2_000)}}
	b.ReportMetric(float64(len(messages[1].Content)), "input-chars")
	for i := 0; i < b.N; i++ {
		if _, err := manager.Build(context.Background(), messages); err != nil {
			b.Fatal(err)
		}
	}
}
