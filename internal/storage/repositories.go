package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func unixMillis(t time.Time) int64 { return t.UTC().UnixMilli() }

func timeFromMillis(value int64) time.Time { return time.UnixMilli(value).UTC() }

func (d *DB) CreateProject(ctx context.Context, project Project) error {
	if project.ID == "" {
		project.ID = NewID()
	}
	if project.CreatedAt.IsZero() {
		project.CreatedAt = time.Now().UTC()
	}
	if project.UpdatedAt.IsZero() {
		project.UpdatedAt = project.CreatedAt
	}
	return d.WriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO projects(id,name,root_path,git_root,pinned,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, project.ID, project.Name, project.RootPath, nullableString(project.GitRoot), project.Pinned, unixMillis(project.CreatedAt), unixMillis(project.UpdatedAt))
		return err
	})
}

const projectColumns = `id,name,root_path,git_root,pinned,created_at,updated_at`

func scanProject(scan func(...any) error) (Project, error) {
	var project Project
	var gitRoot sql.NullString
	var created, updated int64
	if err := scan(&project.ID, &project.Name, &project.RootPath, &gitRoot, &project.Pinned, &created, &updated); err != nil {
		return Project{}, err
	}
	project.GitRoot, project.CreatedAt, project.UpdatedAt = gitRoot.String, timeFromMillis(created), timeFromMillis(updated)
	return project, nil
}

func (d *DB) GetProject(ctx context.Context, projectID string) (Project, error) {
	return scanProject(d.SQL.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE id=?`, projectID).Scan)
}

func (d *DB) GetProjectByRoot(ctx context.Context, root string) (Project, error) {
	return scanProject(d.SQL.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE root_path=?`, root).Scan)
}

// ListProjects 置顶项目排在前面，其余按最近使用倒序。
func (d *DB) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := d.SQL.QueryContext(ctx, `SELECT `+projectColumns+` FROM projects ORDER BY pinned DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		project, err := scanProject(rows.Scan)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (d *DB) RenameProject(ctx context.Context, projectID, name string) error {
	if name == "" {
		return errors.New("storage: project name cannot be empty")
	}
	return d.writeOne(ctx, `UPDATE projects SET name=?,updated_at=? WHERE id=?`, name, unixMillis(time.Now().UTC()), projectID)
}

func (d *DB) SetProjectPinned(ctx context.Context, projectID string, pinned bool) error {
	return d.writeOne(ctx, `UPDATE projects SET pinned=? WHERE id=?`, pinned, projectID)
}

// DeleteProject 依赖外键级联，同时移除该项目下的会话、消息与 turn 历史。
func (d *DB) DeleteProject(ctx context.Context, projectID string) error {
	return d.writeOne(ctx, `DELETE FROM projects WHERE id=?`, projectID)
}

// MoveSession 把会话改挂到另一个项目，用于在草稿对话里重选项目。
func (d *DB) MoveSession(ctx context.Context, sessionID, projectID string) error {
	return d.writeOne(ctx, `UPDATE sessions SET project_id=?,updated_at=? WHERE id=?`, projectID, unixMillis(time.Now().UTC()), sessionID)
}

// writeOne 执行单行写入，受影响行数为 0 时返回 sql.ErrNoRows。
func (d *DB) writeOne(ctx context.Context, query string, args ...any) error {
	return d.WriteTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count == 0 {
			return sql.ErrNoRows
		}
		return nil
	})
}

func (d *DB) CreateSession(ctx context.Context, session Session) error {
	if session.ID == "" {
		session.ID = NewID()
	}
	if session.Status == "" {
		session.Status = SessionIdle
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = session.CreatedAt
	}
	return d.WriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO sessions(id,project_id,title,provider,model,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, session.ID, session.ProjectID, session.Title, session.Provider, session.Model, session.Status, unixMillis(session.CreatedAt), unixMillis(session.UpdatedAt))
		return err
	})
}

func (d *DB) ListSessions(ctx context.Context, projectID string) ([]Session, error) {
	rows, err := d.SQL.QueryContext(ctx, `SELECT id,project_id,title,provider,model,status,created_at,updated_at FROM sessions WHERE project_id=? ORDER BY updated_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var session Session
		var created, updated int64
		if err := rows.Scan(&session.ID, &session.ProjectID, &session.Title, &session.Provider, &session.Model, &session.Status, &created, &updated); err != nil {
			return nil, err
		}
		session.CreatedAt, session.UpdatedAt = timeFromMillis(created), timeFromMillis(updated)
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (d *DB) GetSession(ctx context.Context, sessionID string) (Session, error) {
	var session Session
	var created, updated int64
	err := d.SQL.QueryRowContext(ctx, `SELECT id,project_id,title,provider,model,status,created_at,updated_at FROM sessions WHERE id=?`, sessionID).
		Scan(&session.ID, &session.ProjectID, &session.Title, &session.Provider, &session.Model, &session.Status, &created, &updated)
	if err != nil {
		return Session{}, err
	}
	session.CreatedAt, session.UpdatedAt = timeFromMillis(created), timeFromMillis(updated)
	return session, nil
}

func (d *DB) RenameSession(ctx context.Context, sessionID, title string) error {
	if title == "" {
		return errors.New("storage: session title cannot be empty")
	}
	return d.writeOne(ctx, `UPDATE sessions SET title=?,updated_at=? WHERE id=?`, title, unixMillis(time.Now().UTC()), sessionID)
}

func (d *DB) UpdateSessionProviderModel(ctx context.Context, sessionID, provider, model string) error {
	if provider == "" || model == "" {
		return errors.New("storage: provider and model cannot be empty")
	}
	return d.writeOne(ctx, `UPDATE sessions SET provider=?,model=?,updated_at=? WHERE id=?`, provider, model, unixMillis(time.Now().UTC()), sessionID)
}

// CreateTurnWithUserMessage 原子创建 running turn + 用户消息，并拒绝同会话已有活跃 turn。
func (d *DB) CreateTurnWithUserMessage(ctx context.Context, sessionID, content, provider, model string) (Message, AgentTurn, error) {
	if content == "" {
		return Message{}, AgentTurn{}, errors.New("storage: message cannot be empty")
	}
	now := time.Now().UTC()
	message := Message{ID: NewID(), SessionID: sessionID, Role: RoleUser, Content: content, CreatedAt: now}
	turn := AgentTurn{ID: NewID(), SessionID: sessionID, Status: TurnRunning, StartedAt: now}
	err := d.WriteTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `SELECT id FROM sessions WHERE id=?`, sessionID); err != nil {
			return err
		}
		var activeID string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM agent_turns WHERE session_id=? AND status IN ('queued','running','waiting_approval') LIMIT 1`, sessionID).Scan(&activeID); err == nil {
			return ErrActiveTurn
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO agent_turns(id,session_id,status,started_at) VALUES(?,?,?,?)`, turn.ID, turn.SessionID, turn.Status, unixMillis(turn.StartedAt)); err != nil {
			return err
		}
		message.TurnID = turn.ID
		if _, err := tx.ExecContext(ctx, `INSERT INTO messages(id,session_id,turn_id,role,content,created_at) VALUES(?,?,?,?,?,?)`, message.ID, message.SessionID, message.TurnID, message.Role, message.Content, unixMillis(message.CreatedAt)); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE sessions SET status=?,updated_at=? WHERE id=?`, SessionRunning, unixMillis(now), sessionID)
		return err
	})
	return message, turn, err
}

func (d *DB) UpdateTurnStatus(ctx context.Context, turnID string, status TurnStatus, code, text string) error {
	if !validTurnStatus(status) {
		return fmt.Errorf("storage: invalid turn status %q", status)
	}
	return d.WriteTx(ctx, func(tx *sql.Tx) error {
		var sessionID string
		if err := tx.QueryRowContext(ctx, `SELECT session_id FROM agent_turns WHERE id=?`, turnID).Scan(&sessionID); err != nil {
			return err
		}
		var finished any
		if isTerminal(status) {
			finished = unixMillis(time.Now().UTC())
		}
		if _, err := tx.ExecContext(ctx, `UPDATE agent_turns SET status=?,finished_at=?,error_code=?,error_text=? WHERE id=?`, status, finished, nullableString(code), nullableString(text), turnID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE sessions SET status=?,updated_at=? WHERE id=?`, sessionStatusForTurn(status), unixMillis(time.Now().UTC()), sessionID)
		return err
	})
}

func (d *DB) AddAssistantMessage(ctx context.Context, message Message) error {
	if message.ID == "" {
		message.ID = NewID()
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}
	return d.WriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO messages(id,session_id,turn_id,role,content,created_at) VALUES(?,?,?,?,?,?)`, message.ID, message.SessionID, nullableString(message.TurnID), message.Role, message.Content, unixMillis(message.CreatedAt))
		return err
	})
}

// ListMessages 返回展示顺序的持久化对话快照。
// 流式增量走事件总线；断线或 sequence 空洞后，客户端用此快照重新对齐。
func (d *DB) ListMessages(ctx context.Context, sessionID string) ([]Message, error) {
	rows, err := d.SQL.QueryContext(ctx, `SELECT id,session_id,COALESCE(turn_id,''),role,content,created_at FROM messages WHERE session_id=? ORDER BY created_at,id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		var message Message
		var created int64
		if err := rows.Scan(&message.ID, &message.SessionID, &message.TurnID, &message.Role, &message.Content, &created); err != nil {
			return nil, err
		}
		message.CreatedAt = timeFromMillis(created)
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (d *DB) ListTurns(ctx context.Context, sessionID string) ([]AgentTurn, error) {
	rows, err := d.SQL.QueryContext(ctx, `SELECT id,session_id,status,started_at,finished_at,error_code,error_text FROM agent_turns WHERE session_id=? ORDER BY started_at,id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var turns []AgentTurn
	for rows.Next() {
		var turn AgentTurn
		var started int64
		var finished sql.NullInt64
		var code, text sql.NullString
		if err := rows.Scan(&turn.ID, &turn.SessionID, &turn.Status, &started, &finished, &code, &text); err != nil {
			return nil, err
		}
		turn.StartedAt = timeFromMillis(started)
		if finished.Valid {
			value := timeFromMillis(finished.Int64)
			turn.FinishedAt = &value
		}
		turn.ErrorCode, turn.ErrorText = code.String, text.String
		turns = append(turns, turn)
	}
	return turns, rows.Err()
}

func (d *DB) AddToolCall(ctx context.Context, call ToolCall) error {
	if call.ID == "" {
		call.ID = NewID()
	}
	if call.Status == "" {
		call.Status = ToolPending
	}
	if call.CreatedAt.IsZero() {
		call.CreatedAt = time.Now().UTC()
	}
	if call.UpdatedAt.IsZero() {
		call.UpdatedAt = call.CreatedAt
	}
	return d.WriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO tool_calls(id,turn_id,name,arguments,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, call.ID, call.TurnID, call.Name, call.Arguments, call.Status, unixMillis(call.CreatedAt), unixMillis(call.UpdatedAt))
		return err
	})
}

func (d *DB) AddToolResult(ctx context.Context, result ToolResult) error {
	if result.ID == "" {
		result.ID = NewID()
	}
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now().UTC()
	}
	return d.WriteTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO tool_results(id,tool_call_id,content,success,error_code,truncated,raw_ref,metadata_json,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, result.ID, result.ToolCallID, result.Content, result.Success, result.ErrorCode, result.Truncated, nullableString(result.RawRef), nullableString(result.MetadataJSON), unixMillis(result.CreatedAt)); err != nil {
			return err
		}
		status := ToolCompleted
		if !result.Success {
			status = ToolFailed
		}
		_, err := tx.ExecContext(ctx, `UPDATE tool_calls SET status=?,updated_at=? WHERE id=?`, status, unixMillis(result.CreatedAt), result.ToolCallID)
		return err
	})
}

func (d *DB) AddFileChange(ctx context.Context, change FileChange) error {
	if change.ID == "" {
		change.ID = NewID()
	}
	if change.CreatedAt.IsZero() {
		change.CreatedAt = time.Now().UTC()
	}
	return d.WriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO file_changes(id,turn_id,tool_call_id,path,status,before_hash,after_hash,diff,added_lines,deleted_lines,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, change.ID, change.TurnID, change.ToolCallID, change.Path, change.Status, change.BeforeHash, change.AfterHash, change.Diff, change.AddedLines, change.DeletedLines, unixMillis(change.CreatedAt))
		return err
	})
}

func (d *DB) ListFileChanges(ctx context.Context, turnID string) ([]FileChange, error) {
	rows, err := d.SQL.QueryContext(ctx, `SELECT id,turn_id,tool_call_id,path,status,before_hash,after_hash,diff,added_lines,deleted_lines,created_at FROM file_changes WHERE turn_id=? ORDER BY created_at`, turnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var changes []FileChange
	for rows.Next() {
		var change FileChange
		var created int64
		if err := rows.Scan(&change.ID, &change.TurnID, &change.ToolCallID, &change.Path, &change.Status, &change.BeforeHash, &change.AfterHash, &change.Diff, &change.AddedLines, &change.DeletedLines, &created); err != nil {
			return nil, err
		}
		change.CreatedAt = timeFromMillis(created)
		changes = append(changes, change)
	}
	return changes, rows.Err()
}

func (d *DB) ProjectRootForTurn(ctx context.Context, turnID string) (string, error) {
	var root string
	err := d.SQL.QueryRowContext(ctx, `SELECT p.root_path FROM agent_turns t JOIN sessions s ON s.id=t.session_id JOIN projects p ON p.id=s.project_id WHERE t.id=?`, turnID).Scan(&root)
	return root, err
}

// RecoverInFlight 启动时把未完成 turn/会话标为 interrupted/failed，并把 pending 审批过期，
// 避免崩溃后 UI 一直卡在 running。
func (d *DB) RecoverInFlight(ctx context.Context) error {
	return d.WriteTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE agent_turns SET status='interrupted',finished_at=? WHERE status IN ('queued','running','waiting_approval')`, unixMillis(time.Now().UTC())); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE sessions SET status='failed',updated_at=? WHERE status IN ('running','waiting_approval')`, unixMillis(time.Now().UTC())); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE approvals SET status='expired',resolved_at=? WHERE status='pending'`, unixMillis(time.Now().UTC()))
		return err
	})
}

func (d *DB) SetSetting(ctx context.Context, setting Setting) error {
	if setting.UpdatedAt.IsZero() {
		setting.UpdatedAt = time.Now().UTC()
	}
	return d.WriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value_json,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,updated_at=excluded.updated_at`, setting.Key, setting.ValueJSON, unixMillis(setting.UpdatedAt))
		return err
	})
}

func (d *DB) GetSetting(ctx context.Context, key string) (Setting, error) {
	var setting Setting
	var updated int64
	err := d.SQL.QueryRowContext(ctx, `SELECT key,value_json,updated_at FROM settings WHERE key=?`, key).Scan(&setting.Key, &setting.ValueJSON, &updated)
	if err != nil {
		return Setting{}, err
	}
	setting.UpdatedAt = timeFromMillis(updated)
	return setting, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func validTurnStatus(status TurnStatus) bool {
	switch status {
	case TurnQueued, TurnRunning, TurnWaitingApproval, TurnCompleted, TurnCancelled, TurnFailed, TurnInterrupted:
		return true
	default:
		return false
	}
}

func isTerminal(status TurnStatus) bool {
	return status == TurnCompleted || status == TurnCancelled || status == TurnFailed || status == TurnInterrupted
}

func sessionStatusForTurn(status TurnStatus) SessionStatus {
	switch status {
	case TurnCompleted:
		return SessionCompleted
	case TurnCancelled:
		return SessionCancelled
	case TurnInterrupted, TurnFailed:
		return SessionFailed
	default:
		return SessionRunning
	}
}
