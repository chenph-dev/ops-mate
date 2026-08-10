import { useState } from 'react';
import {
  App as AntdApp,
  Button,
  Input,
  Popconfirm,
  Popover,
  Tooltip,
} from 'antd';
import { useTranslation } from 'react-i18next';
import {
  DeleteOutlined,
  EditOutlined,
  HistoryOutlined,
} from '@ant-design/icons';
import type { convstore } from '@wailsjs/go/models';

interface HistoryPopoverProps {
  conversations: convstore.Conversation[];
  activeSession: string | null;
  onSwitchConversation: (sid: string) => Promise<void>;
  onDeleteConversation: (sid: string) => Promise<void>;
  onRenameConversation: (sid: string, title: string) => Promise<void>;
  onRefreshConversations: () => Promise<void>;
}

/** 历史对话气泡菜单：点击工具栏图标弹出会话列表，支持切换、重命名与删除。 */
export default function HistoryPopover({
  conversations,
  activeSession,
  onSwitchConversation,
  onDeleteConversation,
  onRenameConversation,
  onRefreshConversations,
}: HistoryPopoverProps): React.JSX.Element {
  const { modal } = AntdApp.useApp();
  const { t } = useTranslation('ai');
  const [open, setOpen] = useState(false);

  const handleOpenChange = (next: boolean): void => {
    setOpen(next);
    // 每次打开都拉一次最新会话列表
    if (next) void onRefreshConversations();
  };

  const handleRename = (conv: convstore.Conversation): void => {
    let title = conv.title;
    modal.confirm({
      title: t('history.renameTitle'),
      content: (
        <Input
          autoFocus
          defaultValue={conv.title}
          onChange={(e) => (title = e.target.value)}
        />
      ),
      onOk: async () => {
        const name = title.trim();
        if (!name) return;
        await onRenameConversation(conv.id, name);
      },
    });
  };

  return (
    <Popover
      open={open}
      onOpenChange={handleOpenChange}
      trigger="click"
      placement="bottomRight"
      title={t('history.title')}
      styles={{ content: { padding: 4, width: 240 } }}
      content={
        <div style={{ maxHeight: 300, overflow: 'auto' }}>
          {conversations.length === 0 ? (
            <div
              style={{
                padding: '8px 4px',
                fontSize: 12,
                textAlign: 'center',
                color: 'var(--antd-color-text-secondary)',
              }}
            >
              {t('history.empty')}
            </div>
          ) : (
            conversations.map((conv) => (
              <div
                key={conv.id}
                onClick={() => {
                  setOpen(false);
                  void onSwitchConversation(conv.id);
                }}
                title={conv.title}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 4,
                  padding: '4px',
                  borderRadius: 4,
                  cursor: 'pointer',
                }}
              >
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div
                    style={{
                      fontSize: 12,
                      lineHeight: '18px',
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}
                  >
                    {activeSession === conv.id ? t('history.current') : ''}
                    {conv.title}
                  </div>
                  <div
                    style={{
                      fontSize: 10,
                      lineHeight: '14px',
                      color: 'var(--antd-color-text-secondary)',
                    }}
                  >
                    {new Date(conv.updatedAt * 1000).toLocaleString()}
                  </div>
                </div>
                <Button
                  type="text"
                  size="small"
                  icon={<EditOutlined />}
                  title={t('history.renameBtn')}
                  onClick={(e) => {
                    e.stopPropagation();
                    handleRename(conv);
                  }}
                />
                <Popconfirm
                  title={t('history.deleteConfirm')}
                  onConfirm={(e) => {
                    e?.stopPropagation();
                    void onDeleteConversation(conv.id);
                  }}
                >
                  <Button
                    type="text"
                    size="small"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={(e) => e.stopPropagation()}
                  />
                </Popconfirm>
              </div>
            ))
          )}
        </div>
      }
    >
      <Tooltip title={t('history.title')}>
        <Button type="text" size="small" icon={<HistoryOutlined />} />
      </Tooltip>
    </Popover>
  );
}
