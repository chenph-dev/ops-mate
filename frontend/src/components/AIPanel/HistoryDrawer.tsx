import { Button, Drawer, List, Popconfirm } from "antd";
import { DeleteOutlined } from "@ant-design/icons";
import type { convstore } from "@wailsjs/go/models";

interface HistoryDrawerProps {
  open: boolean;
  onClose: () => void;
  conversations: convstore.Conversation[];
  activeSession: string | null;
  onSwitchConversation: (sid: string) => Promise<void>;
  onDeleteConversation: (sid: string) => Promise<void>;
}

/** 历史对话面板：列出当前主机全部会话，点击切换，支持删除。 */
export default function HistoryDrawer({
  open,
  onClose,
  conversations,
  activeSession,
  onSwitchConversation,
  onDeleteConversation,
}: HistoryDrawerProps): React.JSX.Element {
  return (
    <Drawer
      title="历史对话"
      placement="right"
      width={240}
      open={open}
      onClose={onClose}
      styles={{
        header: { padding: "6px 12px", fontSize: 13, fontWeight: 600 },
        body: { padding: "4px" },
      }}
    >
      <List
        size="small"
        dataSource={conversations}
        locale={{ emptyText: "暂无历史对话" }}
        renderItem={(conv) => (
          <List.Item
            style={{ cursor: "pointer", padding: "4px 4px" }}
            onClick={() => {
              onClose();
              void onSwitchConversation(conv.id);
            }}
            actions={[
              <Popconfirm
                key="delete"
                title="删除该对话？此操作不可恢复。"
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
              </Popconfirm>,
            ]}
          >
            <List.Item.Meta
              title={
                <span style={{ fontSize: 12, lineHeight: "18px" }}>
                  {activeSession === conv.id ? "当前 · " : ""}
                  {conv.title}
                </span>
              }
              description={
                <span style={{ fontSize: 10, lineHeight: "14px" }}>
                  {new Date(conv.updatedAt * 1000).toLocaleString()}
                </span>
              }
            />
          </List.Item>
        )}
      />
    </Drawer>
  );
}
