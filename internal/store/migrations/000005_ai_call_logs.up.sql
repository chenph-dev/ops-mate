-- AI 调用审计：记录每次模型/工具调用的 token 用量、耗时与成败。
-- 由 eino callbacks 在每次组件调用结束时写入（代替原先只打 stdout 的实现）。
CREATE TABLE IF NOT EXISTS ai_call_logs (
    id           TEXT PRIMARY KEY,
    session_id   TEXT,                          -- 可能为空（预留跨会话聚合）
    ts           INTEGER NOT NULL,
    component    TEXT NOT NULL,                 -- 'model' | 'tool'
    name         TEXT NOT NULL,                 -- eino 节点名，如 'llm'
    provider     TEXT,                          -- 实现类型，如 'OpenAI'
    tokens_in    INTEGER NOT NULL DEFAULT 0,
    tokens_out   INTEGER NOT NULL DEFAULT 0,
    tokens_total INTEGER NOT NULL DEFAULT 0,
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    ok           INTEGER NOT NULL DEFAULT 1,    -- 1 成功 / 0 失败
    error        TEXT
    -- 刻意不加 session_id 外键：审计日志独立于会话生命周期，
    -- 删除会话不应连带清掉审计历史，且 session_id 可能为空。
);
CREATE INDEX IF NOT EXISTS idx_ai_call_logs_session ON ai_call_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_ai_call_logs_ts ON ai_call_logs(ts);