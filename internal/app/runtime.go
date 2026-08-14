package app

import (
	"context"
	"errors"

	"github.com/kk-2004/klaude/internal/agent"
	"github.com/kk-2004/klaude/internal/approval"
	"github.com/kk-2004/klaude/internal/event"
	"github.com/kk-2004/klaude/internal/model"
	"github.com/kk-2004/klaude/internal/storage"
	"github.com/kk-2004/klaude/internal/tool"
)

// runTurn assembles a fresh provider, project-bound tool registry, permissions,
// persistence adapter, and event stream for every message. Settings changed in
// the UI therefore apply to the next turn without restarting the app.
func (s *Service) runTurn(ctx context.Context, sessionID, turnID string) error {
	s.mu.RLock()
	composition := s.composition
	locks := s.locks
	s.mu.RUnlock()
	if composition == nil || composition.DB == nil {
		return errors.New("agent runtime is not initialized")
	}
	session, err := composition.DB.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	root, err := composition.DB.ProjectRootForTurn(ctx, turnID)
	if err != nil {
		return err
	}
	projectTools, err := composition.NewProjectTools(root, turnID)
	if err != nil {
		return err
	}
	providerConfig := composition.Config.Config.Provider
	providerConfig.Model = session.Model
	provider, err := s.newConfiguredProvider(providerConfig)
	if err != nil {
		return err
	}
	storedMessages, err := composition.DB.ListMessages(ctx, sessionID)
	if err != nil {
		return err
	}

	dispatcher := tool.Dispatcher{
		Registry:    projectTools.Registry,
		Permissions: composition.Permissions,
		Approvals:   composition.Approvals,
		SessionID:   sessionID,
		TurnID:      turnID,
		ApprovalRequested: func(eventContext context.Context, request approval.Request) error {
			if err := composition.DB.UpdateTurnStatus(eventContext, turnID, storage.TurnWaitingApproval, "", ""); err != nil {
				return err
			}
			payload := map[string]any{"approval": map[string]any{
				"id": request.ID, "summary": request.Summary, "cwd": root,
				"hash": request.RequestHash, "risk": request.Risk,
			}}
			return composition.Events.Publish(eventContext, sessionID, turnID, event.ApprovalRequired, payload)
		},
		ApprovalResolved: func(eventContext context.Context) error {
			return composition.DB.UpdateTurnStatus(eventContext, turnID, storage.TurnRunning, "", "")
		},
	}

	runtime := agent.Agent{
		Provider:    provider,
		Context:     composition.Context,
		Dispatcher:  agentToolDispatcher{dispatcher: dispatcher},
		Events:      composition.Events,
		Store:       turnStore{db: composition.DB, sessionID: sessionID, turnID: turnID},
		SessionID:   sessionID,
		TurnID:      turnID,
		ProjectID:   session.ProjectID,
		MaxTurns:    composition.Config.Config.Agent.MaxTurns,
		Messages:    modelMessages(storedMessages),
		Mutating:    true,
		Locks:       locks,
		ScheduleCfg: composition.SchedulerConfigFromApp(),
		ToolMeta:    projectTools.Lookup,
		Tools:       modelToolDefinitions(projectTools.Registry.Definitions()),
	}
	return runtime.Run(ctx)
}

type agentToolDispatcher struct{ dispatcher tool.Dispatcher }

func (d agentToolDispatcher) Dispatch(ctx context.Context, call agent.ToolCall) (agent.ToolResult, error) {
	result, err := d.dispatcher.Dispatch(ctx, call.Name, call.Arguments)
	return agent.ToolResult{Content: result.Content, Success: result.Success, ErrorCode: result.ErrorCode, Truncated: result.Truncated, RawRef: result.RawRef}, err
}

type turnStore struct {
	db        *storage.DB
	sessionID string
	turnID    string
}

func (s turnStore) AppendAssistant(ctx context.Context, content string) error {
	return s.db.AddAssistantMessage(ctx, storage.Message{SessionID: s.sessionID, TurnID: s.turnID, Role: storage.RoleAssistant, Content: content})
}

func (s turnStore) AppendTool(ctx context.Context, call agent.ToolCall, result agent.ToolResult) error {
	if err := s.db.AddToolCall(ctx, storage.ToolCall{ID: call.ID, TurnID: s.turnID, Name: call.Name, Arguments: string(call.Arguments), Status: storage.ToolRunning}); err != nil {
		return err
	}
	return s.db.AddToolResult(ctx, storage.ToolResult{ToolCallID: call.ID, Content: result.Content, Success: result.Success, ErrorCode: result.ErrorCode, Truncated: result.Truncated, RawRef: result.RawRef})
}

func (s turnStore) SetStatus(ctx context.Context, status agent.Status, cause error) error {
	code, text := "", ""
	if cause != nil {
		code, text = "agent_error", cause.Error()
	}
	return s.db.UpdateTurnStatus(ctx, s.turnID, storage.TurnStatus(status), code, text)
}

func modelMessages(messages []storage.Message) []model.Message {
	result := make([]model.Message, 0, len(messages))
	for _, message := range messages {
		result = append(result, model.Message{Role: model.Role(message.Role), Content: message.Content})
	}
	return result
}

func modelToolDefinitions(definitions []tool.Definition) []model.ToolDefinition {
	result := make([]model.ToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		result = append(result, model.ToolDefinition{Name: definition.Name, Description: definition.Description, Parameters: definition.Parameters})
	}
	return result
}
