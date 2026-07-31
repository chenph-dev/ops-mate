package store

const schemaSQL = `
CREATE TABLE IF NOT EXISTS hosts (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    node_type     TEXT NOT NULL DEFAULT 'host',  -- 'folder' | 'host'
    parent_id     TEXT,                           -- NULL = 根级
    addr          TEXT,
    port          INTEGER,
    user          TEXT,
    auth_encrypted BLOB,
    auth_type     TEXT,
    created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_config (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    provider          TEXT NOT NULL,
    model             TEXT NOT NULL,
    base_url          TEXT NOT NULL,
    api_key_encrypted BLOB
);

CREATE TABLE IF NOT EXISTS conversations (
    id         TEXT PRIMARY KEY,
    host_id    TEXT NOT NULL,
    title      TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS messages (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL,
    role        TEXT NOT NULL,
    content     TEXT NOT NULL,
    tool_result TEXT,
    ts          INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES conversations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS commands (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    command    TEXT NOT NULL,
    exit_code   INTEGER,
    output     TEXT,
    ts         INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES conversations(id) ON DELETE CASCADE
);

CREATE VIRTUAL TABLE IF NOT EXISTS commands_fts USING fts5(
    command, output, content='commands', content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS commands_ai AFTER INSERT ON commands BEGIN
    INSERT INTO commands_fts(rowid, command, output)
    VALUES (new.rowid, new.command, COALESCE(new.output, ''));
END;
CREATE TRIGGER IF NOT EXISTS commands_ad AFTER DELETE ON commands BEGIN
    DELETE FROM commands_fts WHERE rowid = old.rowid;
END;
`
