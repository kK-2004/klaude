package agent

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"
)

func TestLayerizeSamePathWritesSerialize(t *testing.T) {
	calls := []ToolCall{
		{ID: "a", Name: "write_file", Arguments: json.RawMessage(`{"path":"f.go","content":"1"}`)},
		{ID: "b", Name: "write_file", Arguments: json.RawMessage(`{"path":"f.go","content":"2"}`)},
	}
	plan := BuildSchedule(calls, nil, true, nil)
	if plan.Serial {
		t.Fatalf("unexpected serial: %s", plan.Reason)
	}
	if len(plan.Layers) != 2 {
		t.Fatalf("layers = %v, want 2", plan.Layers)
	}
}

func TestLayerizeDifferentPathWritesParallel(t *testing.T) {
	calls := []ToolCall{
		{ID: "a", Name: "write_file", Arguments: json.RawMessage(`{"path":"a.go","content":"1"}`)},
		{ID: "b", Name: "write_file", Arguments: json.RawMessage(`{"path":"b.go","content":"2"}`)},
	}
	plan := BuildSchedule(calls, nil, true, nil)
	if plan.Serial || len(plan.Layers) != 1 || len(plan.Layers[0]) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestShellBarrierSplitsLayers(t *testing.T) {
	calls := []ToolCall{
		{ID: "r", Name: "read_file", Arguments: json.RawMessage(`{"path":"a.go"}`)},
		{ID: "s", Name: "shell", Arguments: json.RawMessage(`{"program":"echo"}`)},
		{ID: "w", Name: "write_file", Arguments: json.RawMessage(`{"path":"b.go","content":"x"}`)},
	}
	// Without planner edges, ambiguous shell forces serial fallback.
	plan := BuildSchedule(calls, nil, true, nil)
	if !plan.Serial {
		t.Fatalf("expected serial for ambiguous shell without planner, got %+v", plan)
	}
	// With empty validated extra from planner path via BuildSchedule after hard-only when we force non-ambiguous... 
	// Use PlanToolBatch with fake planner proposing no edges — still ambiguous without edges.
	// Provide planner edges that are empty: still ambiguous => serial in PlanToolBatch when LLM returns empty?
	// Build hard edges alone with shell: classify shell as Ambiguous; BuildSchedule without extra serializes.
	// Force layerize on hard edges only:
	nodes := make([]SchedNode, len(calls))
	for i, call := range calls {
		nodes[i] = ClassifyCall(i, call, defaultMeta(call.Name))
	}
	layers, ok := Layerize(nodes, BuildHardEdges(nodes))
	if !ok || len(layers) != 3 {
		t.Fatalf("barrier layers = %v ok=%v", layers, ok)
	}
}

func TestWideReadMutexWithWrites(t *testing.T) {
	calls := []ToolCall{
		{ID: "g", Name: "grep", Arguments: json.RawMessage(`{"pattern":"foo"}`)},
		{ID: "w", Name: "write_file", Arguments: json.RawMessage(`{"path":"a.go","content":"x"}`)},
	}
	plan := BuildSchedule(calls, nil, true, nil)
	if plan.Serial || len(plan.Layers) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestValidateExtraEdgesRejectsReverseAndCycle(t *testing.T) {
	calls := []ToolCall{
		{ID: "a", Name: "write_file", Arguments: json.RawMessage(`{"path":"f.go","content":"1"}`)},
		{ID: "b", Name: "write_file", Arguments: json.RawMessage(`{"path":"f.go","content":"2"}`)},
	}
	nodes := []SchedNode{
		ClassifyCall(0, calls[0], defaultMeta(calls[0].Name)),
		ClassifyCall(1, calls[1], defaultMeta(calls[1].Name)),
	}
	hard := BuildHardEdges(nodes)
	if err := ValidateExtraEdges(nodes, hard, []SchedEdge{{From: "b", To: "a"}}); err == nil {
		t.Fatal("expected reverse rejection")
	}
	if err := ValidateExtraEdges(nodes, hard, []SchedEdge{{From: "a", To: "b"}, {From: "b", To: "a"}}); err == nil {
		t.Fatal("expected cycle rejection")
	}
}

type fakePlanner struct {
	edges []SchedEdge
	err   error
}

func (p fakePlanner) ProposeEdges(context.Context, []SchedNode, []SchedEdge) ([]SchedEdge, error) {
	return p.edges, p.err
}

func TestPlanToolBatchLLMAcceptAndReject(t *testing.T) {
	calls := []ToolCall{
		{ID: "s", Name: "shell", Arguments: json.RawMessage(`{"program":"true"}`)},
		{ID: "r", Name: "read_file", Arguments: json.RawMessage(`{"path":"a.go"}`)},
	}
	okPlan := PlanToolBatch(context.Background(), calls, nil, SchedulerConfig{ParallelTools: true, LLMSchedule: true}, fakePlanner{edges: []SchedEdge{{From: "s", To: "r"}}})
	if okPlan.Serial {
		t.Fatalf("expected accepted planner edges, got %s", okPlan.Reason)
	}
	bad := PlanToolBatch(context.Background(), calls, nil, SchedulerConfig{ParallelTools: true, LLMSchedule: true}, fakePlanner{edges: []SchedEdge{{From: "r", To: "s"}}})
	if !bad.Serial {
		t.Fatalf("expected serial fallback, got %+v", bad)
	}
}

func TestExecuteLayersRunsParallel(t *testing.T) {
	var concurrent int32
	var maxConcurrent int32
	dispatcher := &blockingDispatcher{onDispatch: func() {
		cur := atomic.AddInt32(&concurrent, 1)
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if cur <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&concurrent, -1)
	}}
	calls := []ToolCall{{ID: "1", Name: "read_file"}, {ID: "2", Name: "read_file"}}
	plan := SchedulePlan{
		Nodes:  []SchedNode{{Index: 0, Call: calls[0], Concurrent: true}, {Index: 1, Call: calls[1], Concurrent: true}},
		Layers: [][]int{{0, 1}},
	}
	results := ExecuteLayers(context.Background(), dispatcher, calls, plan)
	if len(results) != 2 || atomic.LoadInt32(&maxConcurrent) < 2 {
		t.Fatalf("maxConcurrent=%d results=%+v", maxConcurrent, results)
	}
}

type blockingDispatcher struct {
	onDispatch func()
}

func (d *blockingDispatcher) Dispatch(_ context.Context, call ToolCall) (ToolResult, error) {
	if d.onDispatch != nil {
		d.onDispatch()
	}
	return ToolResult{Content: call.ID, Success: true}, nil
}

func TestParseScheduleEdges(t *testing.T) {
	edges, err := parseScheduleEdges(`Here you go: {"edges":[{"from":"a","to":"b"}]} thanks`)
	if err != nil || len(edges) != 1 || edges[0].From != "a" {
		t.Fatalf("edges=%v err=%v", edges, err)
	}
}
