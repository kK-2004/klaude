package agent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/kk-2004/klaude/internal/model"
	"github.com/kk-2004/klaude/internal/trace"
)

// SchedulePlanner proposes additional ordering edges for ambiguous tool batches.
type SchedulePlanner interface {
	ProposeEdges(ctx context.Context, nodes []SchedNode, hard []SchedEdge) ([]SchedEdge, error)
}

// ProviderSchedulePlanner asks the model for JSON edges; failures return an error for serial fallback.
type ProviderSchedulePlanner struct {
	Provider model.Provider
	Timeout  time.Duration
}

func (p ProviderSchedulePlanner) ProposeEdges(ctx context.Context, nodes []SchedNode, hard []SchedEdge) ([]SchedEdge, error) {
	if p.Provider == nil {
		return nil, errSchedule("schedule planner provider is nil")
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	planCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	prompt := buildSchedulePrompt(nodes, hard)
	stream, err := p.Provider.Stream(planCtx, model.Request{
		Messages: []model.Message{{Role: model.RoleUser, Content: prompt}},
	})
	if err != nil {
		return nil, err
	}
	text, _, _, _, err := consume(planCtx, stream)
	if err != nil {
		return nil, err
	}
	return parseScheduleEdges(text)
}

func buildSchedulePrompt(nodes []SchedNode, hard []SchedEdge) string {
	var b strings.Builder
	b.WriteString("Propose additional tool execution order edges as JSON only.\n")
	b.WriteString(`Schema: {"edges":[{"from":"tool_id","to":"tool_id"}]}\n`)
	b.WriteString("from must complete before to. Do not reverse existing hard edges. Do not invent ids.\n\nTools:\n")
	for _, node := range nodes {
		args := trace.RedactString(string(node.Call.Arguments))
		if len(args) > 400 {
			args = args[:400] + "..."
		}
		b.WriteString("- id=")
		b.WriteString(node.Call.ID)
		b.WriteString(" name=")
		b.WriteString(node.Call.Name)
		b.WriteString(" kind=")
		b.WriteString(string(node.Kind))
		b.WriteString(" ambiguous=")
		if node.Ambiguous {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString(" paths=")
		b.WriteString(strings.Join(node.Paths, ","))
		b.WriteString(" args=")
		b.WriteString(args)
		b.WriteByte('\n')
	}
	b.WriteString("\nHard edges:\n")
	for _, edge := range hard {
		b.WriteString("- ")
		b.WriteString(edge.From)
		b.WriteString(" -> ")
		b.WriteString(edge.To)
		b.WriteByte('\n')
	}
	return b.String()
}

type scheduleEdgePayload struct {
	Edges []SchedEdge `json:"edges"`
}

func parseScheduleEdges(text string) ([]SchedEdge, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, errSchedule("empty planner response")
	}
	// Extract first JSON object if the model wrapped it in prose.
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return nil, errSchedule("planner response is not JSON")
	}
	trimmed = trimmed[start : end+1]
	var payload scheduleEdgePayload
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, errSchedule("invalid planner JSON")
	}
	return dedupeEdges(payload.Edges), nil
}

// PlanToolBatch builds a schedule, optionally consulting a planner when dependencies are ambiguous.
func PlanToolBatch(ctx context.Context, calls []ToolCall, lookup ToolMetaLookup, cfg SchedulerConfig, planner SchedulePlanner) SchedulePlan {
	if !cfg.ParallelTools {
		return BuildSchedule(calls, lookup, false, nil)
	}
	nodes := make([]SchedNode, len(calls))
	ambiguous := false
	for i, call := range calls {
		meta := defaultMeta(call.Name)
		if lookup != nil {
			meta = lookup.Lookup(call.Name)
		}
		nodes[i] = ClassifyCall(i, call, meta)
		if nodes[i].Ambiguous {
			ambiguous = true
		}
	}
	hard := BuildHardEdges(nodes)
	var extra []SchedEdge
	if ambiguous && cfg.LLMSchedule && planner != nil {
		proposed, err := planner.ProposeEdges(ctx, nodes, hard)
		if err != nil {
			return BuildSchedule(calls, lookup, false, nil).withReason("planner failed: " + err.Error())
		}
		if err := ValidateExtraEdges(nodes, hard, proposed); err != nil {
			return BuildSchedule(calls, lookup, false, nil).withReason("planner edges rejected: " + err.Error())
		}
		extra = proposed
	}
	return BuildSchedule(calls, lookup, true, extra)
}

func (p SchedulePlan) withReason(reason string) SchedulePlan {
	p.Reason = reason
	p.Serial = true
	return p
}
