-- 审批策略：单行配置（id=1）。enable_auto 控制全局自动放行开关；
-- readonly_whitelist 逗号分隔的只读命令白名单（空 = 使用内置默认白名单）。
CREATE TABLE IF NOT EXISTS approval_policy (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    enable_auto        INTEGER NOT NULL DEFAULT 1,
    readonly_whitelist TEXT    NOT NULL DEFAULT ''
);

-- 主机级覆盖：'inherit' 继承全局 / 'on' 允许 / 'off' 禁止。
-- 已有主机自动补 'inherit'，行为与升级前一致（升级前即全量审批，等用户显式开启）。
ALTER TABLE hosts ADD COLUMN auto_approve TEXT NOT NULL DEFAULT 'inherit';
