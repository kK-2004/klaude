package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const workspaceWide = "workspace:*"

type SchedulerConfig struct {
	ParallelTools bool
	LLMSchedule   bool
}

type NodeKind string

const (
	KindRead    NodeKind = "read"
	KindWrite   NodeKind = "write"
	KindBarrier NodeKind = "barrier"
)

// ToolMeta describes concurrency/safety hints used by the scheduler.
type ToolMeta struct {
	ReadOnly   bool
	Concurrent bool
	Known      bool // false => unregistered / unknown tool
}

// ToolMetaLookup resolves tool metadata for scheduling; optional on Agent.
type ToolMetaLookup interface {
	Lookup(name string) ToolMeta
}

type SchedEdge struct {
	From string
	To   string
}

type SchedNode struct {
	Index      int
	Call       ToolCall
	Kind       NodeKind
	Paths      []string
	Concurrent bool
	Ambiguous  bool
}

type SchedulePlan struct {
	Nodes      []SchedNode
	Layers     [][]int // indices into Nodes / original calls
	Serial     bool
	Reason     string
	ExtraEdges []SchedEdge
}

// ClassifyCall builds a SchedNode from a tool call and optional registry metadata.
func ClassifyCall(index int, call ToolCall, meta ToolMeta) SchedNode {
	node := SchedNode{Index: index, Call: call, Concurrent: meta.Concurrent}
	name := strings.TrimSpace(call.Name)
	var fields map[string]any
	_ = json.Unmarshal(call.Arguments, &fields)
	path, _ := fields["path"].(string)
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." {
		path = ""
	}

	switch name {
	case "read_file", "list_directory":
		node.Kind = KindRead
		if path == "" {
			node.Ambiguous = true
			node.Paths = []string{workspaceWide}
		} else {
			node.Paths = []string{path}
		}
		if !meta.Known {
			node.Ambiguous = true
		}
	case "grep", "glob":
		node.Kind = KindRead
		if path != "" {
			node.Paths = []string{path}
		} else {
			node.Paths = []string{workspaceWide}
		}
		node.Concurrent = true
		if !meta.Known {
			node.Ambiguous = true
		}
	case "write_file", "apply_patch":
		node.Kind = KindWrite
		node.Concurrent = false
		if path == "" {
			node.Ambiguous = true
			node.Paths = []string{workspaceWide}
		} else {
			node.Paths = []string{path}
		}
		if !meta.Known {
			node.Ambiguous = true
		}
	case "shell":
		node.Kind = KindBarrier
		node.Ambiguous = true
		node.Paths = nil
	default:
		node.Kind = KindBarrier
		node.Ambiguous = true
		if path != "" {
			node.Paths = []string{path}
		}
		if !meta.Known {
			node.Ambiguous = true
		}
	}
	return node
}

// BuildHardEdges returns ordering constraints that cannot be relaxed by an LLM.
func BuildHardEdges(nodes []SchedNode) []SchedEdge {
	var edges []SchedEdge
	add := func(from, to int) {
		if from == to {
			return
		}
		edges = append(edges, SchedEdge{From: nodes[from].Call.ID, To: nodes[to].Call.ID})
	}

	// Same path: preserve batch order.
	byPath := map[string][]int{}
	for i, node := range nodes {
		for _, path := range node.Paths {
			if path == "" {
				continue
			}
			byPath[path] = append(byPath[path], i)
		}
	}
	for _, indexes := range byPath {
		for i := 0; i+1 < len(indexes); i++ {
			add(indexes[i], indexes[i+1])
		}
	}

	// write(path) must precede later reads/writes on the same path (covered by same-path order
	// when both share the path). Also: workspace-wide reads mutex with all writes.
	wideReads := []int{}
	writes := []int{}
	for i, node := range nodes {
		if node.Kind == KindWrite {
			writes = append(writes, i)
		}
		for _, path := range node.Paths {
			if path == workspaceWide && node.Kind == KindRead {
				wideReads = append(wideReads, i)
			}
		}
	}
	for _, r := range wideReads {
		for _, w := range writes {
			if r < w {
				add(r, w)
			} else if w < r {
				add(w, r)
			}
		}
	}

	// Barriers: everything before must finish first; everything after must wait.
	for b, node := range nodes {
		if node.Kind != KindBarrier {
			continue
		}
		for i := range nodes {
			if i < b {
				add(i, b)
			} else if i > b {
				add(b, i)
			}
		}
	}
	return dedupeEdges(edges)
}

func dedupeEdges(edges []SchedEdge) []SchedEdge {
	seen := map[string]struct{}{}
	out := make([]SchedEdge, 0, len(edges))
	for _, edge := range edges {
		if edge.From == "" || edge.To == "" || edge.From == edge.To {
			continue
		}
		key := edge.From + "->" + edge.To
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, edge)
	}
	return out
}

// Layerize runs Kahn topological layering. On cycle, returns serial layers and ok=false.
func Layerize(nodes []SchedNode, edges []SchedEdge) (layers [][]int, ok bool) {
	idToIndex := map[string]int{}
	for i, node := range nodes {
		idToIndex[node.Call.ID] = i
	}
	indegree := make([]int, len(nodes))
	adj := make([][]int, len(nodes))
	for _, edge := range edges {
		from, okFrom := idToIndex[edge.From]
		to, okTo := idToIndex[edge.To]
		if !okFrom || !okTo {
			continue
		}
		adj[from] = append(adj[from], to)
		indegree[to]++
	}

	remaining := len(nodes)
	current := make([]int, 0, len(nodes))
	for i, deg := range indegree {
		if deg == 0 {
			current = append(current, i)
		}
	}
	for len(current) > 0 {
		sort.Ints(current)
		layers = append(layers, append([]int(nil), current...))
		next := make([]int, 0)
		for _, from := range current {
			remaining--
			for _, to := range adj[from] {
				indegree[to]--
				if indegree[to] == 0 {
					next = append(next, to)
				}
			}
		}
		current = next
	}
	if remaining != 0 {
		serial := make([][]int, len(nodes))
		for i := range nodes {
			serial[i] = []int{i}
		}
		return serial, false
	}
	return layers, true
}

// ValidateExtraEdges ensures LLM edges do not violate hard rules or introduce cycles.
func ValidateExtraEdges(nodes []SchedNode, hard, extra []SchedEdge) error {
	idToIndex := map[string]int{}
	for i, node := range nodes {
		idToIndex[node.Call.ID] = i
	}
	hardSet := map[string]struct{}{}
	for _, edge := range hard {
		hardSet[edge.From+"->"+edge.To] = struct{}{}
	}
	merged := append([]SchedEdge{}, hard...)
	for _, edge := range extra {
		if edge.From == "" || edge.To == "" {
			return errSchedule("empty edge endpoint")
		}
		from, okFrom := idToIndex[edge.From]
		to, okTo := idToIndex[edge.To]
		if !okFrom || !okTo {
			return errSchedule("edge references unknown tool id")
		}
		if from == to {
			return errSchedule("self-edge")
		}
		// Reject reversing an existing hard edge.
		if _, ok := hardSet[edge.To+"->"+edge.From]; ok {
			return errSchedule("extra edge reverses hard constraint")
		}
		// Same-path order: cannot place later call before earlier call on same path.
		if sharesPath(nodes[from], nodes[to]) && from > to {
			return errSchedule("extra edge breaks same-path order")
		}
		// Cannot move a node across a barrier incorrectly: if hard already forbids, caught by reverse check / cycle.
		merged = append(merged, edge)
	}
	if _, ok := Layerize(nodes, merged); !ok {
		return errSchedule("merged edges contain a cycle")
	}
	return nil
}

func sharesPath(a, b SchedNode) bool {
	set := map[string]struct{}{}
	for _, path := range a.Paths {
		set[path] = struct{}{}
	}
	for _, path := range b.Paths {
		if _, ok := set[path]; ok {
			return true
		}
	}
	return false
}

type scheduleError string

func (e scheduleError) Error() string { return string(e) }
func errSchedule(msg string) error    { return scheduleError("agent schedule: " + msg) }

// BuildSchedule classifies calls, optionally merges planner edges, and returns layers.
func BuildSchedule(calls []ToolCall, lookup ToolMetaLookup, parallel bool, extra []SchedEdge) SchedulePlan {
	nodes := make([]SchedNode, len(calls))
	ambiguous := false
	for i, call := range calls {
		meta := ToolMeta{Known: false}
		if lookup != nil {
			meta = lookup.Lookup(call.Name)
		} else {
			meta = defaultMeta(call.Name)
		}
		nodes[i] = ClassifyCall(i, call, meta)
		if nodes[i].Ambiguous {
			ambiguous = true
		}
	}
	if !parallel {
		layers := make([][]int, len(nodes))
		for i := range nodes {
			layers[i] = []int{i}
		}
		return SchedulePlan{Nodes: nodes, Layers: layers, Serial: true, Reason: "parallel_tools disabled"}
	}
	hard := BuildHardEdges(nodes)
	if len(extra) > 0 {
		if err := ValidateExtraEdges(nodes, hard, extra); err != nil {
			layers := make([][]int, len(nodes))
			for i := range nodes {
				layers[i] = []int{i}
			}
			return SchedulePlan{Nodes: nodes, Layers: layers, Serial: true, Reason: err.Error(), ExtraEdges: extra}
		}
		hard = dedupeEdges(append(hard, extra...))
	} else if ambiguous {
		layers := make([][]int, len(nodes))
		for i := range nodes {
			layers[i] = []int{i}
		}
		return SchedulePlan{Nodes: nodes, Layers: layers, Serial: true, Reason: "ambiguous dependencies without validated planner edges"}
	}
	layers, ok := Layerize(nodes, hard)
	if !ok {
		return SchedulePlan{Nodes: nodes, Layers: layers, Serial: true, Reason: "cycle in dependency graph", ExtraEdges: extra}
	}
	return SchedulePlan{Nodes: nodes, Layers: layers, Serial: false, ExtraEdges: extra}
}

func defaultMeta(name string) ToolMeta {
	switch name {
	case "read_file", "list_directory", "grep", "glob":
		return ToolMeta{ReadOnly: true, Concurrent: true, Known: true}
	case "write_file", "apply_patch":
		return ToolMeta{ReadOnly: false, Concurrent: false, Known: true}
	case "shell":
		return ToolMeta{ReadOnly: false, Concurrent: false, Known: true}
	default:
		return ToolMeta{Known: false}
	}
}

// ExecuteLayers runs each layer; within a non-serial layer all calls run concurrently.
// Results are returned aligned to the original call order.
func ExecuteLayers(ctx context.Context, dispatcher Dispatcher, calls []ToolCall, plan SchedulePlan) []BatchResult {
	results := make([]BatchResult, len(calls))
	for i, call := range calls {
		results[i].Call = call
	}
	if dispatcher == nil {
		for i := range results {
			results[i].Err = errSchedule("dispatcher is not configured")
			results[i].Result = ToolResult{ErrorCode: "tool_unavailable"}
		}
		return results
	}
	for _, layer := range plan.Layers {
		parallelLayer := !plan.Serial && len(layer) > 1
		var wait sync.WaitGroup
		for _, index := range layer {
			call := plan.Nodes[index].Call
			if parallelLayer {
				wait.Add(1)
				go func(index int, call ToolCall) {
					defer wait.Done()
					results[index].Result, results[index].Err = dispatcher.Dispatch(ctx, call)
				}(index, call)
			} else {
				results[index].Result, results[index].Err = dispatcher.Dispatch(ctx, call)
			}
		}
		wait.Wait()
	}
	return results
}
