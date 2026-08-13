package permission

import "testing"

func TestDefaultPolicyAndHardBoundaries(t *testing.T) {
	engine := NewDefault()
	if decision, _ := engine.Evaluate(Request{ToolName: "read_file", ReadOnly: true}); decision != Allow {
		t.Fatalf("read = %s", decision)
	}
	if decision, _ := engine.Evaluate(Request{ToolName: "apply_patch"}); decision != Ask {
		t.Fatalf("write = %s", decision)
	}
	if decision, _ := engine.Evaluate(Request{ToolName: "read_file", ReadOnly: true, Outside: true}); decision != Deny {
		t.Fatalf("outside = %s", decision)
	}
	if decision, _ := engine.Evaluate(Request{ToolName: "shell", Command: "sudo rm -rf ."}); decision != Deny {
		t.Fatalf("forbidden = %s", decision)
	}
}

func TestRulePrecedence(t *testing.T) {
	engine := NewDefault()
	engine.Rules = []Rule{{Pattern: "git status", Decision: Allow}}
	if decision, _ := engine.Evaluate(Request{ToolName: "shell", Command: "git status --short"}); decision != Allow {
		t.Fatalf("rule decision = %s", decision)
	}
}
