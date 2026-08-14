package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	agentcontext "github.com/kk-2004/klaude/internal/context"
	"github.com/kk-2004/klaude/internal/event"
	"github.com/kk-2004/klaude/internal/model"
)

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type ToolResult struct {
	Content   string
	Success   bool
	ErrorCode string
	Truncated bool
	RawRef    string
}

type Dispatcher interface {
	Dispatch(context.Context, ToolCall) (ToolResult, error)
}

type Store interface {
	AppendAssistant(context.Context, string) error
	AppendTool(context.Context, ToolCall, ToolResult) error
	SetStatus(context.Context, Status, error) error
}

type Status string

const (
	StatusQueued          Status = "queued"
	StatusRunning         Status = "running"
	StatusWaitingApproval Status = "waiting_approval"
	StatusCompleted       Status = "completed"
	StatusCancelled       Status = "cancelled"
	StatusFailed          Status = "failed"
	StatusInterrupted     Status = "interrupted"
)

type Agent struct {
	Provider    model.Provider
	Context     agentcontext.Manager
	Dispatcher  Dispatcher
	Events      *event.Bus
	Store       Store
	SessionID   string
	TurnID      string
	ProjectID   string
	MaxTurns    int
	Messages    []model.Message
	Mutating    bool
	Locks       *MutationLocks
	ScheduleCfg SchedulerConfig
	ToolMeta    ToolMetaLookup
	Tools       []model.ToolDefinition
	Planner     SchedulePlanner
	mu          sync.Mutex
}

// Run 执行一次 Agent turn 的主循环：
// 1. 可选地按项目加互斥锁（写操作互斥）
// 2. 组装上下文并流式调用模型
// 3. 若有 tool call 则逐个派发，结果写回 Messages 供下一轮使用
// 4. 无 tool call 且模型正常结束则完成；触达 MaxTurns 则失败
func (a *Agent) Run(ctx context.Context) error {
	if a.Provider == nil {
		return errors.New("agent: provider is not configured")
	}
	if a.Events == nil {
		a.Events = event.NewBus()
	}
	if a.MaxTurns <= 0 {
		a.MaxTurns = 50
	}
	var release func()
	// 同一项目上的变更型 Agent 互斥，避免并发写盘互相踩踏。
	if a.Mutating && a.Locks != nil {
		var acquired bool
		var err error
		acquired, release, err = a.Locks.Acquire(ctx, a.ProjectID)
		if err != nil {
			return err
		}
		if !acquired {
			return errors.New("agent: project mutation lock is unavailable")
		}
		defer release()
	}
	if err := a.setStatus(ctx, StatusRunning, nil); err != nil {
		return err
	}
	if err := a.Events.Publish(ctx, a.SessionID, a.TurnID, event.AgentStarted, nil); err != nil {
		return err
	}
	for turn := 0; turn < a.MaxTurns; turn++ {
		request, err := a.Context.Build(ctx, a.Messages)
		if err != nil {
			return a.fail(ctx, "context_limit", err)
		}
		request.Tools = a.Tools
		stream, err := a.streamWithRetry(ctx, request)
		if err != nil {
			if errors.Is(err, context.Canceled) || model.IsCancelled(err) {
				return a.cancel(ctx)
			}
			return a.fail(ctx, "provider_error", err)
		}
		text, calls, usage, completed, err := consume(ctx, stream)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return a.cancel(ctx)
			}
			return a.fail(ctx, "stream_error", err)
		}
		if text != "" {
			a.Messages = append(a.Messages, model.Message{Role: model.RoleAssistant, Content: text})
			if a.Store != nil {
				if err := a.Store.AppendAssistant(ctx, text); err != nil {
					return a.fail(ctx, "storage_error", err)
				}
			}
			if err := a.Events.Publish(ctx, a.SessionID, a.TurnID, event.TextDelta, map[string]string{"text": text}); err != nil {
				return err
			}
		}
		if usage != nil {
			if err := a.Events.Publish(ctx, a.SessionID, a.TurnID, event.UsageUpdated, usage); err != nil {
				return err
			}
		}
		// 模型声明完成且无工具调用 → turn 正常结束。
		if len(calls) == 0 && completed {
			return a.complete(ctx)
		}
		if len(calls) == 0 {
			return a.fail(ctx, "incomplete_model_response", errors.New("model stream ended without completion"))
		}
		if a.Dispatcher == nil {
			return a.fail(ctx, "tool_unavailable", errors.New("agent: tool dispatcher is not configured"))
		}
		planner := a.Planner
		if planner == nil && a.ScheduleCfg.ParallelTools && a.ScheduleCfg.LLMSchedule {
			planner = ProviderSchedulePlanner{Provider: a.Provider}
		}
		plan := PlanToolBatch(ctx, calls, a.ToolMeta, a.ScheduleCfg, planner)
		for _, call := range calls {
			if err := a.Events.Publish(ctx, a.SessionID, a.TurnID, event.ToolStarted, map[string]string{"id": call.ID, "name": call.Name}); err != nil {
				return err
			}
		}
		batch := ExecuteLayers(ctx, a.Dispatcher, calls, plan)
		for _, item := range batch {
			result, dispatchErr := item.Result, item.Err
			if dispatchErr != nil && result.ErrorCode == "" {
				result.ErrorCode = "tool_error"
			}
			if a.Store != nil {
				if err := a.Store.AppendTool(ctx, item.Call, result); err != nil {
					return a.fail(ctx, "storage_error", err)
				}
			}
			if err := a.Events.Publish(ctx, a.SessionID, a.TurnID, event.ToolFinished, result); err != nil {
				return err
			}
			// 工具错误也写入对话上下文，让模型在下一轮自行纠错/换策略。
			a.Messages = append(a.Messages, model.Message{Role: model.RoleTool, Content: result.Content, ToolCallID: item.Call.ID, Name: item.Call.Name})
		}
	}
	return a.fail(ctx, "max_turns", errors.New("agent: maximum turns reached"))
}

// MutationLocks 按 projectID 提供容量为 1 的信道锁，实现「同项目同时只允许一个变更型 Agent」。
type MutationLocks struct {
	mu    sync.Mutex
	locks map[string]chan struct{}
}

func NewMutationLocks() *MutationLocks { return &MutationLocks{locks: make(map[string]chan struct{})} }
func (l *MutationLocks) Acquire(ctx context.Context, projectID string) (bool, func(), error) {
	if projectID == "" {
		return true, func() {}, nil
	}
	l.mu.Lock()
	lock := l.locks[projectID]
	if lock == nil {
		lock = make(chan struct{}, 1)
		l.locks[projectID] = lock
	}
	l.mu.Unlock()
	select {
	case lock <- struct{}{}:
		return true, func() { <-lock }, nil
	case <-ctx.Done():
		return false, nil, ctx.Err()
	}
}

type BatchCall struct {
	Call       ToolCall
	Concurrent bool
}
type BatchResult struct {
	Call   ToolCall
	Result ToolResult
	Err    error
}

// RunReadBatch 按 Concurrent 标记并行或串行派发只读工具；结果下标与输入对齐。
func RunReadBatch(ctx context.Context, dispatcher Dispatcher, calls []BatchCall) []BatchResult {
	results := make([]BatchResult, len(calls))
	var wait sync.WaitGroup
	for index, item := range calls {
		results[index].Call = item.Call
		if item.Concurrent {
			wait.Add(1)
			go func(index int, call ToolCall) {
				defer wait.Done()
				results[index].Result, results[index].Err = dispatcher.Dispatch(ctx, call)
			}(index, item.Call)
		} else {
			results[index].Result, results[index].Err = dispatcher.Dispatch(ctx, item.Call)
		}
	}
	wait.Wait()
	return results
}

// streamWithRetry 仅对标记为 Retryable 的模型错误做指数退避重试（最多 3 次）。
func (a *Agent) streamWithRetry(ctx context.Context, request model.Request) (<-chan model.Event, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		stream, err := a.Provider.Stream(ctx, request)
		if err == nil {
			return stream, nil
		}
		last = err
		var providerErr *model.Error
		if !errors.As(err, &providerErr) || !providerErr.Retryable {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeAfter(attempt):
		}
	}
	return nil, last
}

func (a *Agent) setStatus(ctx context.Context, status Status, cause error) error {
	if a.Store == nil {
		return nil
	}
	return a.Store.SetStatus(ctx, status, cause)
}
func (a *Agent) complete(ctx context.Context) error {
	if err := a.setStatus(ctx, StatusCompleted, nil); err != nil {
		return err
	}
	return a.Events.Publish(ctx, a.SessionID, a.TurnID, event.AgentFinished, nil)
}
func (a *Agent) cancel(ctx context.Context) error {
	_ = a.setStatus(ctx, StatusCancelled, context.Canceled)
	_ = a.Events.Publish(ctx, a.SessionID, a.TurnID, event.AgentCancelled, nil)
	return context.Canceled
}
func (a *Agent) fail(ctx context.Context, code string, err error) error {
	_ = a.setStatus(ctx, StatusFailed, err)
	_ = a.Events.Publish(ctx, a.SessionID, a.TurnID, event.AgentError, map[string]string{"code": code, "message": err.Error()})
	return err
}

// consume 把模型 SSE/流式事件拼成完整文本与工具调用列表。
// 工具参数按 ID 增量拼接；非法 JSON 降级为 null，避免中断整轮。
func consume(ctx context.Context, stream <-chan model.Event) (string, []ToolCall, any, bool, error) {
	var text string
	var usage any
	var calls []ToolCall
	partial := make(map[string]*ToolCall)
	completed := false
	for {
		select {
		case <-ctx.Done():
			return text, calls, usage, false, ctx.Err()
		case item, ok := <-stream:
			if !ok {
				return text, calls, usage, completed, nil
			}
			switch item.Type {
			case model.TextDelta:
				text += item.Text
			case model.ToolCallStart:
				partial[item.ID] = &ToolCall{ID: item.ID, Name: item.Name}
			case model.ToolCallDelta:
				if call := partial[item.ID]; call != nil {
					call.Arguments = append(call.Arguments, item.Data...)
				}
			case model.ToolCallEnd:
				call := partial[item.ID]
				if call == nil {
					call = &ToolCall{ID: item.ID, Name: item.Name}
				}
				if len(item.Arguments) > 0 {
					call.Arguments = append([]byte(nil), item.Arguments...)
				}
				if !json.Valid(call.Arguments) {
					calls = append(calls, ToolCall{ID: call.ID, Name: call.Name, Arguments: json.RawMessage(`null`)})
				} else {
					calls = append(calls, *call)
				}
			case model.UsageUpdate:
				usage = item
			case model.ModelCompleted:
				completed = true
				return text, calls, usage, completed, nil
			}
		}
	}
}

func timeAfter(attempt int) <-chan time.Time {
	return time.After(time.Duration(1<<attempt) * 50 * time.Millisecond)
}
