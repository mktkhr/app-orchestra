CREATE TABLE IF NOT EXISTS workspaces (
    id    TEXT PRIMARY KEY,
    name  TEXT NOT NULL,
    owner TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS panels (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    service      TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    component    TEXT NOT NULL,
    title        TEXT NOT NULL,
    args         TEXT NOT NULL,
    position     INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_panels_workspace_position ON panels (workspace_id, position);

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL UNIQUE,
    role          TEXT NOT NULL,
    password_hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS permissions (
    user_id      TEXT NOT NULL,
    service      TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    PRIMARY KEY (user_id, service, operation_id)
);
