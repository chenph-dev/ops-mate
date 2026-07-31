import { Button, Tag, Divider, theme } from 'antd';
import { WarningOutlined } from '@ant-design/icons';
import type { CommandSuggestion } from '@/hooks/useSessions';

interface CommandCardProps {
  command: CommandSuggestion;
  onApprove: () => void;
  onReject: () => void;
}

export default function CommandCard({ command, onApprove, onReject }: CommandCardProps) {
  const { token } = theme.useToken();
  const isHighRisk = command.assessedRisk === 'high' || command.risk === 'high';

  return (
    <div
      style={{
        border: `1px solid ${isHighRisk ? token.colorError : token.colorWarning}`,
        borderRadius: 8,
        padding: 12,
        background: token.colorBgElevated,
        margin: '8px 0',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
        <WarningOutlined style={{ color: isHighRisk ? token.colorError : token.colorWarning }} />
        <span style={{ fontWeight: 600, fontSize: 13 }}>
          {isHighRisk ? '⚠️ AI 提议执行高风险命令' : 'AI 提议执行命令'}
        </span>
      </div>

      <div
        style={{
          fontFamily: '"Cascadia Code", "Fira Code", "Consolas", monospace',
          fontSize: 12,
          background: token.colorFillSecondary,
          padding: '8px 10px',
          borderRadius: 4,
          marginBottom: 8,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
        }}
      >
        $ {command.command}
      </div>

      <div style={{ fontSize: 12, color: token.colorTextSecondary }}>
        <div>原因: {command.why || '-'}</div>
        <div style={{ marginTop: 4 }}>
          风险: {' '}
          <Tag color={isHighRisk ? 'red' : 'orange'}>
            {isHighRisk ? '高' : command.risk || '低'}
          </Tag>
        </div>
      </div>

      <Divider style={{ margin: '8px 0' }} />

      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
        <Button size="small" onClick={onReject}>拒绝</Button>
        <Button type="primary" size="small" danger={isHighRisk} onClick={onApprove}>
          批准执行
        </Button>
      </div>
    </div>
  );
}
