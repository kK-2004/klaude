package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/klaude/klaude/internal/approval"
	"github.com/klaude/klaude/internal/permission"
	"github.com/klaude/klaude/internal/trace"
)

// Dispatcher 是工具执行的统一入口：校验参数 → 权限裁决 →（必要时）阻塞等人批 → 真正执行。
type Dispatcher struct {
	Registry    *Registry
	Permissions permission.Engine
	Approvals   *approval.Manager
	SessionID   string
	TurnID      string
}

func (d Dispatcher) Dispatch(ctx context.Context, name string, arguments json.RawMessage) (Result, error) {
	item, ok := d.Registry.Get(name)
	if !ok {
		return Result{ErrorCode: "unknown_tool"}, fmt.Errorf("tool %q is not registered", name)
	}
	definition := item.Definition()
	if err := ValidateArguments(definition, arguments); err != nil {
		return Result{ErrorCode: "invalid_arguments"}, err
	}
	request := permission.Request{ToolName: name, ReadOnly: definition.Metadata.ReadOnly, Destructive: definition.Metadata.Destructive}
	var fields map[string]any
	_ = json.Unmarshal(arguments, &fields)
	if path, _ := fields["path"].(string); path != "" {
		request.Path = path
	}
	if program, _ := fields["program"].(string); program != "" {
		request.Command = strings.TrimSpace(program + " " + strings.Join(stringSlice(fields["args"]), " "))
	}
	decision, reason := d.Permissions.Evaluate(request)
	if decision == permission.Deny {
		return Result{ErrorCode: "permission_denied", Content: reason}, nil
	}
	// Ask / RequiresApproval：创建审批请求并阻塞，直到用户批准或拒绝。
	if decision == permission.Ask || definition.Metadata.RequiresApproval {
		if d.Approvals == nil {
			return Result{ErrorCode: "approval_unavailable"}, errors.New("approval manager is not configured")
		}
		summary := trace.RedactString(string(arguments))
		pending := d.Approvals.Create(approval.Request{SessionID: d.SessionID, TurnID: d.TurnID, ToolName: name, Summary: summary, Risk: risk(definition.Metadata), RequestHash: approval.Hash(summary)})
		resolution, err := d.Approvals.Wait(ctx, pending.ID)
		if err != nil {
			return Result{ErrorCode: "approval_cancelled"}, err
		}
		if resolution.Status != approval.Approved {
			return Result{ErrorCode: "permission_rejected", Content: "user rejected the operation"}, nil
		}
	}
	result, err := item.Execute(ctx, arguments)
	if err != nil && result.ErrorCode == "" {
		result.ErrorCode = "tool_error"
	}
	return result, err
}

func stringSlice(value any) []string {
	raw, _ := value.([]any)
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}
func risk(metadata Metadata) string {
	if metadata.Destructive {
		return "high"
	}
	if metadata.RequiresApproval {
		return "medium"
	}
	return "low"
}
