package permission

import (
	"errors"
	"strings"
)

type Decision string

const (
	Allow Decision = "allow"
	Ask   Decision = "ask"
	Deny  Decision = "deny"
)

type Request struct {
	ToolName    string
	Path        string
	Command     string
	ReadOnly    bool
	Destructive bool
	Outside     bool
}

type Rule struct {
	Pattern  string
	Decision Decision
}

// Engine 按「越界/黑名单 → 显式规则 → 读写/Shell 默认策略」裁决一次工具请求。
type Engine struct {
	Read  Decision
	Write Decision
	Shell Decision
	Rules []Rule
}

func NewDefault() Engine { return Engine{Read: Allow, Write: Ask, Shell: Ask, Rules: nil} }

func (e Engine) Evaluate(request Request) (Decision, string) {
	if request.Outside {
		return Deny, "workspace boundary"
	}
	if isForbidden(request) {
		return Deny, "explicitly forbidden operation"
	}
	for _, rule := range e.Rules {
		if rule.Pattern != "" && (strings.HasPrefix(request.Command, rule.Pattern) || request.ToolName == rule.Pattern || strings.HasPrefix(request.Path, rule.Pattern)) {
			if rule.Decision == Allow || rule.Decision == Ask || rule.Decision == Deny {
				return rule.Decision, "matched rule: " + rule.Pattern
			}
			return Deny, "invalid permission rule"
		}
	}
	if request.ReadOnly {
		return defaultDecision(e.Read, Allow), "default read policy"
	}
	if request.ToolName == "shell" || request.Command != "" {
		return defaultDecision(e.Shell, Ask), "default shell policy"
	}
	return defaultDecision(e.Write, Ask), "default write policy"
}

// isForbidden 硬拒绝高危命令前缀（如 rm / sudo），不进入 Ask 流程。
func isForbidden(request Request) bool {
	command := strings.TrimSpace(strings.ToLower(request.Command))
	return strings.HasPrefix(command, "rm ") || command == "rm" || strings.HasPrefix(command, "sudo ") || command == "sudo"
}

func defaultDecision(value, fallback Decision) Decision {
	if value == Allow || value == Ask || value == Deny {
		return value
	}
	return fallback
}

func ParseDecision(value string) (Decision, error) {
	switch Decision(strings.ToLower(strings.TrimSpace(value))) {
	case Allow, Ask, Deny:
		return Decision(strings.ToLower(strings.TrimSpace(value))), nil
	default:
		return "", errors.New("permission decision must be allow, ask, or deny")
	}
}
