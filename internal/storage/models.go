package storage

import "time"

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RootPath  string    `json:"rootPath"`
	GitRoot   string    `json:"gitRoot,omitempty"`
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SessionStatus string

const (
	SessionIdle            SessionStatus = "idle"
	SessionRunning         SessionStatus = "running"
	SessionWaitingApproval SessionStatus = "waiting_approval"
	SessionCancelled       SessionStatus = "cancelled"
	SessionFailed          SessionStatus = "failed"
	SessionCompleted       SessionStatus = "completed"
)

type Session struct {
	ID        string        `json:"id"`
	ProjectID string        `json:"projectId"`
	Title     string        `json:"title"`
	Provider  string        `json:"provider"`
	Model     string        `json:"model"`
	Status    SessionStatus `json:"status"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

type Message struct {
	ID        string      `json:"id"`
	SessionID string      `json:"sessionId"`
	TurnID    string      `json:"turnId,omitempty"`
	Role      MessageRole `json:"role"`
	Content   string      `json:"content"`
	CreatedAt time.Time   `json:"createdAt"`
}

type TurnStatus string

const (
	TurnQueued          TurnStatus = "queued"
	TurnRunning         TurnStatus = "running"
	TurnWaitingApproval TurnStatus = "waiting_approval"
	TurnCompleted       TurnStatus = "completed"
	TurnCancelled       TurnStatus = "cancelled"
	TurnFailed          TurnStatus = "failed"
	TurnInterrupted     TurnStatus = "interrupted"
)

type AgentTurn struct {
	ID         string     `json:"id"`
	SessionID  string     `json:"sessionId"`
	Status     TurnStatus `json:"status"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	ErrorCode  string     `json:"errorCode,omitempty"`
	ErrorText  string     `json:"errorText,omitempty"`
}

type ToolCallStatus string

const (
	ToolPending   ToolCallStatus = "pending"
	ToolRunning   ToolCallStatus = "running"
	ToolCompleted ToolCallStatus = "completed"
	ToolFailed    ToolCallStatus = "failed"
	ToolCancelled ToolCallStatus = "cancelled"
)

type ToolCall struct {
	ID        string         `json:"id"`
	TurnID    string         `json:"turnId"`
	Name      string         `json:"name"`
	Arguments string         `json:"arguments"`
	Status    ToolCallStatus `json:"status"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type ToolResult struct {
	ID           string    `json:"id"`
	ToolCallID   string    `json:"toolCallId"`
	Content      string    `json:"content"`
	Success      bool      `json:"success"`
	ErrorCode    string    `json:"errorCode,omitempty"`
	Truncated    bool      `json:"truncated"`
	RawRef       string    `json:"rawRef,omitempty"`
	MetadataJSON string    `json:"metadataJson,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ApprovalStatus string

const (
	ApprovalPending   ApprovalStatus = "pending"
	ApprovalApproved  ApprovalStatus = "approved"
	ApprovalRejected  ApprovalStatus = "rejected"
	ApprovalCancelled ApprovalStatus = "cancelled"
	ApprovalExpired   ApprovalStatus = "expired"
)

type Approval struct {
	ID          string         `json:"id"`
	TurnID      string         `json:"turnId"`
	ToolCallID  string         `json:"toolCallId"`
	ToolName    string         `json:"toolName"`
	Summary     string         `json:"summary"`
	WorkingDir  string         `json:"workingDir"`
	Risk        string         `json:"risk"`
	RequestHash string         `json:"requestHash"`
	Status      ApprovalStatus `json:"status"`
	CreatedAt   time.Time      `json:"createdAt"`
	ResolvedAt  *time.Time     `json:"resolvedAt,omitempty"`
}

type FileChange struct {
	ID           string    `json:"id"`
	TurnID       string    `json:"turnId"`
	ToolCallID   string    `json:"toolCallId"`
	Path         string    `json:"path"`
	Status       string    `json:"status"`
	BeforeHash   string    `json:"beforeHash"`
	AfterHash    string    `json:"afterHash"`
	Diff         string    `json:"diff"`
	AddedLines   int       `json:"addedLines"`
	DeletedLines int       `json:"deletedLines"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Usage struct {
	ID              string    `json:"id"`
	TurnID          string    `json:"turnId"`
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
	InputTokens     *int      `json:"inputTokens,omitempty"`
	CachedTokens    *int      `json:"cachedTokens,omitempty"`
	OutputTokens    *int      `json:"outputTokens,omitempty"`
	ReasoningTokens *int      `json:"reasoningTokens,omitempty"`
	CostCents       *int64    `json:"costCents,omitempty"`
	LatencyMS       int64     `json:"latencyMs"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Setting struct {
	Key       string    `json:"key"`
	ValueJSON string    `json:"valueJson"`
	UpdatedAt time.Time `json:"updatedAt"`
}
