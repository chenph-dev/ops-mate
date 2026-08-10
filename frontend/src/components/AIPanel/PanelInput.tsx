import { Button, Dropdown, Input } from 'antd';
import { SendOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { Ref } from 'react';
import type { InputRef } from 'antd';

/** 斜杠快捷命令菜单：新增命令只需在此追加。desc 为 ai 命名空间下的 i18n key。 */
const SLASH_COMMANDS: { name: string; desc: string }[] = [
  { name: '/clear', desc: 'slash.clearDesc' },
  { name: '/new', desc: 'slash.newDesc' },
];

interface PanelInputProps {
  /** 输入框 ref（示例引导填充后聚焦用）。 */
  ref?: Ref<InputRef>;
  input: string;
  sending: boolean;
  inputDisabled: boolean;
  configured: boolean;
  onInputChange: (value: string) => void;
  onSend: () => Promise<void>;
  onKeyDown: (e: React.KeyboardEvent) => void;
  /** 点击斜杠命令菜单项时回调（AIPanel 执行对应快捷命令）。 */
  onSlashCommand?: (cmd: string) => void;
}

/**
 * 抽屉底部输入区。
 * 发送按钮使用 antd Input 原生的 suffix 内置在输入框内部右侧，
 * 天然与输入框一体，不存在外部按钮的高度对齐问题。
 */
export default function PanelInput({
  ref,
  input,
  sending,
  inputDisabled,
  configured,
  onInputChange,
  onSend,
  onKeyDown,
  onSlashCommand,
}: PanelInputProps): React.JSX.Element {
  const { t } = useTranslation('ai');
  // 输入以 / 开头时弹出斜杠快捷命令菜单
  const showCmdMenu = input.startsWith('/');

  // 斜杠命令菜单项样式（组件内构建以随语言刷新）
  const slashMenuItems = SLASH_COMMANDS.map((cmd) => ({
    key: cmd.name,
    label: (
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <span
          style={{
            fontFamily: 'monospace',
            color: 'var(--antd-color-primary)',
          }}
        >
          {cmd.name}
        </span>
        <span
          style={{ color: 'var(--antd-color-text-secondary)', fontSize: 12 }}
        >
          {t(cmd.desc)}
        </span>
      </div>
    ),
  }));

  return (
    <div
      style={{
        padding: '6px 8px',
        borderTop: '1px solid var(--antd-color-border-secondary)',
        flexShrink: 0,
      }}
    >
      {/* Dropdown 渲染到 portal，不受父容器 overflow:hidden 裁剪 */}
      <Dropdown
        open={showCmdMenu}
        trigger={['click']}
        placement="bottomLeft"
        menu={{
          items: slashMenuItems,
          onClick: ({ key }) => onSlashCommand?.(key),
        }}
      >
        <Input
          ref={ref}
          value={input}
          onChange={(e) => onInputChange(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder={
            !configured
              ? t('input.placeholderConfig')
              : inputDisabled
                ? t('input.placeholderWait')
                : t('input.placeholder')
          }
          disabled={inputDisabled}
          style={{ fontSize: 12 }}
          suffix={
            <Button
              type="primary"
              size="small"
              icon={<SendOutlined />}
              onClick={() => void onSend()}
              disabled={!input.trim() || sending || inputDisabled}
              loading={sending}
              style={{ marginRight: -8 }}
            />
          }
        />
      </Dropdown>
    </div>
  );
}
