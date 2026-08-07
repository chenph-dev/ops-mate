-- 占位迁移：历史遗留的 command_embeddings 功能已回滚（见 git 历史），
-- 但既有数据库的 schema_migrations 已记录「版本 4 已应用」。
-- 此 no-op 用于对齐版本号，让 golang-migrate 能继续应用后续迁移（000005 等）。
SELECT 1;