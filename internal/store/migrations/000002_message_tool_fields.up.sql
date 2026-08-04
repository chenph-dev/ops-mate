-- messages 表增加 tool calling 字段，支持忠实还原 assistant tool_calls / tool 结果配对。
ALTER TABLE messages ADD COLUMN tool_calls TEXT;
ALTER TABLE messages ADD COLUMN tool_call_id TEXT;
ALTER TABLE messages ADD COLUMN tool_name TEXT;
