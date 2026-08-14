package context

import (
	stdcontext "context"
	"errors"
	"fmt"
	"strings"

	"github.com/kk-2004/klaude/internal/model"
)

// Manager 按字符预算裁剪对话：系统指令优先保留，历史从最近往前装，
// 工具结果可单独截断，并为模型输出预留 OutputReserveChars。
type Manager struct {
	SystemInstructions  string
	ProjectInstructions string
	BudgetChars         int
	OutputReserveChars  int
	ToolResultChars     int
}

var ErrBudgetExceeded = errors.New("context: required content exceeds budget")

func (m Manager) Build(_ stdcontext.Context, messages []model.Message) (model.Request, error) {
	budget := m.BudgetChars
	if budget <= 0 {
		budget = 120_000
	}
	reserve := m.OutputReserveChars
	if reserve <= 0 {
		reserve = budget / 10
		if reserve > 12_000 {
			reserve = 12_000
		}
	}
	available := budget - reserve
	if available <= 0 {
		return model.Request{}, ErrBudgetExceeded
	}
	instructions := make([]model.Message, 0, 2)
	for _, content := range []string{m.SystemInstructions, m.ProjectInstructions} {
		if strings.TrimSpace(content) != "" {
			instructions = append(instructions, model.Message{Role: model.RoleSystem, Content: content})
		}
	}
	conversation := make([]model.Message, 0, len(messages))
	used := 0
	// 从最新消息向前塞，直到预算用尽；至少保留一条，否则报超限。
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		content := message.Content
		if message.Role == model.RoleTool && m.ToolResultChars > 0 && len(content) > m.ToolResultChars {
			content = truncate(content, m.ToolResultChars)
		}
		if used+len(content) > available {
			if len(conversation) == 0 {
				return model.Request{}, fmt.Errorf("%w: message %d is too large", ErrBudgetExceeded, index)
			}
			break
		}
		conversation = append([]model.Message{{Role: message.Role, Content: content, ToolCallID: message.ToolCallID, Name: message.Name}}, conversation...)
		used += len(content)
	}
	result := append(instructions, conversation...)
	return model.Request{Messages: result}, nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	marker := "...[truncated]..."
	if limit <= len(marker) {
		return marker[:limit]
	}
	if limit < 32 {
		return value[:limit-len(marker)] + marker
	}
	heads := limit / 2
	tail := limit - heads - len(marker)
	return value[:heads] + marker + value[len(value)-tail:]
}
