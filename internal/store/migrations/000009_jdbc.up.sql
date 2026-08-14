-- JDBC 数据库连接支持：driver 数据库驱动（mysql/postgres）；database 目标库名。
ALTER TABLE hosts ADD COLUMN driver TEXT NOT NULL DEFAULT 'mysql';
ALTER TABLE hosts ADD COLUMN database TEXT NOT NULL DEFAULT '';
