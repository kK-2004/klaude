ALTER TABLE projects ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_projects_pinned_updated ON projects(pinned DESC, updated_at DESC);
