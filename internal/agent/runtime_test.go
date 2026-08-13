package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	agentcontext "github.com/klaude/klaude/internal/context"
	"github.com/klaude/klaude/internal/event"
	"github.com/klaude/klaude/internal/model"
)

type fakeProvider struct {
	streams [][]model.Event
	calls   int
}

func (p *fakeProvider) Stream(_ context.Context, _ model.Request) (<-chan model.Event, error) {
	if p.calls >= len(p.streams) {
		return nil, &model.Error{Code: "missing_fixture", Message: "no stream fixture"}
	}
	events := p.streams[p.calls]
	p.calls++
	channel := make(chan model.Event, len(events))
	for _, item := range events {
		channel <- item
	}
	close(channel)
	return channel, nil
}

type fakeDispatcher struct {
	mu     sync.Mutex
	calls  []ToolCall
	result ToolResult
}

func (d *fakeDispatcher) Dispatch(_ context.Context, call ToolCall) (ToolResult, error) {
	d.mu.Lock()
	d.calls = append(d.calls, call)
	d.mu.Unlock()
	return d.result, nil
}

type fakeStore struct {
	statuses  []Status
	assistant []string
	tools     int
}

func (s *fakeStore) AppendAssistant(_ context.Context, content string) error {
	s.assistant = append(s.assistant, content)
	return nil
}
func (s *fakeStore) AppendTool(_ context.Context, _ ToolCall, _ ToolResult) error {
	s.tools++
	return nil
}
func (s *fakeStore) SetStatus(_ context.Context, status Status, _ error) error {
	s.statuses = append(s.statuses, status)
	return nil
}

func TestAgentCompletesTextTurnAndEmitsOrderedEvents(t *testing.T) {
	provider := &fakeProvider{streams: [][]model.Event{{{Type: model.TextDelta, Text: "hello"}, {Type: model.ModelCompleted}}}}
	store := &fakeStore{}
	bus := event.NewBus()
	var got []event.Envelope
	bus.Subscribe(func(_ context.Context, envelope event.Envelope) error { got = append(got, envelope); return nil })
	agent := Agent{Provider: provider, Context: agentcontext.Manager{BudgetChars: 1000}, Events: bus, Store: store, SessionID: "s", TurnID: "t"}
	if err := agent.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.assistant) != 1 || store.assistant[0] != "hello" {
		t.Fatalf("assistant = %+v", store.assistant)
	}
	if store.statuses[len(store.statuses)-1] != StatusCompleted {
		t.Fatalf("statuses = %+v", store.statuses)
	}
	if len(got) != 3 || got[0].Type != event.AgentStarted || got[2].Type != event.AgentFinished {
		t.Fatalf("events = %+v", got)
	}
}

func TestAgentExecutesToolThenContinues(t *testing.T) {
	callArgs := json.RawMessage(`{"path":"main.go"}`)
	provider := &fakeProvider{streams: [][]model.Event{
		{{Type: model.ToolCallStart, ID: "call-1", Name: "read_file"}, {Type: model.ToolCallEnd, ID: "call-1", Arguments: callArgs}, {Type: model.ModelCompleted}},
		{{Type: model.TextDelta, Text: "done"}, {Type: model.ModelCompleted}},
	}}
	dispatcher := &fakeDispatcher{result: ToolResult{Content: "package main", Success: true}}
	agent := Agent{Provider: provider, Context: agentcontext.Manager{BudgetChars: 1000}, Dispatcher: dispatcher, Events: event.NewBus(), SessionID: "s", TurnID: "t"}
	if err := agent.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.calls) != 1 || string(dispatcher.calls[0].Arguments) != string(callArgs) {
		t.Fatalf("calls = %+v", dispatcher.calls)
	}
}

func TestAgentCancellationStopsStream(t *testing.T) {
	provider := &blockingProvider{}
	ctx, cancel := context.WithCancel(context.Background())
	agent := Agent{Provider: provider, Context: agentcontext.Manager{BudgetChars: 1000}, Events: event.NewBus(), SessionID: "s", TurnID: "t"}
	done := make(chan error, 1)
	go func() { done <- agent.Run(ctx) }()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestMutationLocksSerializeProjects(t *testing.T) {
	locks := NewMutationLocks()
	first, release, err := locks.Acquire(context.Background(), "project")
	if err != nil || !first {
		t.Fatalf("first lock: acquired=%v err=%v", first, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	secondDone := make(chan error, 1)
	go func() { _, _, err := locks.Acquire(ctx, "project"); secondDone <- err }()
	cancel()
	if err := <-secondDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("second lock error = %v", err)
	}
	release()
}

func TestRunReadBatchExecutesConcurrentCalls(t *testing.T) {
	dispatcher := &fakeDispatcher{result: ToolResult{Content: "ok", Success: true}}
	results := RunReadBatch(context.Background(), dispatcher, []BatchCall{{Call: ToolCall{ID: "1"}, Concurrent: true}, {Call: ToolCall{ID: "2"}, Concurrent: true}})
	if len(results) != 2 || len(dispatcher.calls) != 2 {
		t.Fatalf("results=%+v calls=%+v", results, dispatcher.calls)
	}
}

func TestAgentParallelToolsDispatchesBatch(t *testing.T) {
	callArgs := json.RawMessage(`{"path":"a.go"}`)
	callArgs2 := json.RawMessage(`{"path":"b.go"}`)
	provider := &fakeProvider{streams: [][]model.Event{
		{
			{Type: model.ToolCallStart, ID: "1", Name: "read_file"}, {Type: model.ToolCallEnd, ID: "1", Arguments: callArgs},
			{Type: model.ToolCallStart, ID: "2", Name: "read_file"}, {Type: model.ToolCallEnd, ID: "2", Arguments: callArgs2},
			{Type: model.ModelCompleted},
		},
		{{Type: model.TextDelta, Text: "done"}, {Type: model.ModelCompleted}},
	}}
	dispatcher := &fakeDispatcher{result: ToolResult{Content: "ok", Success: true}}
	agent := Agent{
		Provider: provider, Context: agentcontext.Manager{BudgetChars: 1000}, Dispatcher: dispatcher, Events: event.NewBus(),
		SessionID: "s", TurnID: "t", ScheduleCfg: SchedulerConfig{ParallelTools: true},
	}
	if err := agent.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.calls) != 2 {
		t.Fatalf("calls = %+v", dispatcher.calls)
	}
}

func TestAgentRetriesRetryableProviderError(t *testing.T) {
	provider := &retryProvider{}
	agent := Agent{Provider: provider, Context: agentcontext.Manager{BudgetChars: 1000}, Events: event.NewBus(), SessionID: "s", TurnID: "t"}
	if err := agent.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.calls)
	}
}

type blockingProvider struct{}

func (*blockingProvider) Stream(ctx context.Context, _ model.Request) (<-chan model.Event, error) {
	channel := make(chan model.Event)
	go func() { <-ctx.Done(); close(channel) }()
	return channel, nil
}

type retryProvider struct{ calls int }

func (p *retryProvider) Stream(_ context.Context, _ model.Request) (<-chan model.Event, error) {
	p.calls++
	if p.calls == 1 {
		return nil, &model.Error{Code: "rate_limit", Message: "try again", Retryable: true}
	}
	channel := make(chan model.Event, 1)
	channel <- model.Event{Type: model.ModelCompleted}
	close(channel)
	return channel, nil
}
