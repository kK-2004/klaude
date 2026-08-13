CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  root_path TEXT NOT NULL UNIQUE,
  git_root TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('idle','running','waiting_approval','cancelled','failed','completed')),
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_project_updated ON sessions(project_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  turn_id TEXT REFERENCES agent_turns(id) ON DELETE SET NULL,
  role TEXT NOT NULL CHECK (role IN ('system','user','assistant','tool')),
  content TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_session_created ON messages(session_id, created_at);

CREATE TABLE IF NOT EXISTS agent_turns (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK (status IN ('queued','running','waiting_approval','completed','cancelled','failed','interrupted')),
  started_at INTEGER NOT NULL,
  finished_at INTEGER,
  error_code TEXT,
  error_text TEXT
);
CREATE INDEX IF NOT EXISTS idx_turns_session_started ON agent_turns(session_id, started_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_turns_one_active_session
  ON agent_turns(session_id) WHERE status IN ('queued','running','waiting_approval');

CREATE TABLE IF NOT EXISTS tool_calls (
  id TEXT PRIMARY KEY,
  turn_id TEXT NOT NULL REFERENCES agent_turns(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  arguments TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending','running','completed','failed','cancelled')),
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tool_calls_turn_created ON tool_calls(turn_id, created_at);

CREATE TABLE IF NOT EXISTS tool_results (
  id TEXT PRIMARY KEY,
  tool_call_id TEXT NOT NULL REFERENCES tool_calls(id) ON DELETE CASCADE,
  content TEXT NOT NULL,
  success INTEGER NOT NULL,
  error_code TEXT,
  truncated INTEGER NOT NULL DEFAULT 0,
  raw_ref TEXT,
  metadata_json TEXT,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS approvals (
  id TEXT PRIMARY KEY,
  turn_id TEXT NOT NULL REFERENCES agent_turns(id) ON DELETE CASCADE,
  tool_call_id TEXT NOT NULL REFERENCES tool_calls(id) ON DELETE CASCADE,
  tool_name TEXT NOT NULL,
  summary TEXT NOT NULL,
  working_dir TEXT NOT NULL,
  risk TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending','approved','rejected','cancelled','expired')),
  created_at INTEGER NOT NULL,
  resolved_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_approvals_pending ON approvals(status, created_at);

CREATE TABLE IF NOT EXISTS file_changes (
  id TEXT PRIMARY KEY,
  turn_id TEXT NOT NULL REFERENCES agent_turns(id) ON DELETE CASCADE,
  tool_call_id TEXT NOT NULL REFERENCES tool_calls(id) ON DELETE CASCADE,
  path TEXT NOT NULL,
  status TEXT NOT NULL,
  before_hash TEXT NOT NULL,
  after_hash TEXT NOT NULL,
  diff TEXT NOT NULL,
  added_lines INTEGER NOT NULL,
  deleted_lines INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_file_changes_turn_created ON file_changes(turn_id, created_at);

CREATE TABLE IF NOT EXISTS usage (
  id TEXT PRIMARY KEY,
  turn_id TEXT NOT NULL REFERENCES agent_turns(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  input_tokens INTEGER,
  cached_tokens INTEGER,
  output_tokens INTEGER,
  reasoning_tokens INTEGER,
  cost_cents INTEGER,
  latency_ms INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value_json TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);
