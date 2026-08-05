-- messages 表增加审批状态字段：tool 消息落库时明确记录该命令的审批结果
-- （"approved" 已批准执行 / "rejected" 已拒绝）。前端历史回放直接读此字段
-- 显示审批状态，无需从 tool 消息文案推断。
ALTER TABLE messages ADD COLUMN approval_status TEXT;
