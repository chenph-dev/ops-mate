import { Button, List, Tooltip, Popconfirm, theme } from "antd";
import { PlusOutlined, DeleteOutlined, LinkOutlined } from "@ant-design/icons";
import type { hoststore } from "@wailsjs/go/models";

type HostMeta = hoststore.HostMeta;

interface HostListProps {
  hosts: HostMeta[];
  selectedId: string | null;
  onSelect: (host: HostMeta) => void;
  onAdd: () => void;
  onDelete: (id: string) => void;
  onTest: (host: HostMeta) => void;
}

export default function HostList({
  hosts,
  selectedId,
  onSelect,
  onAdd,
  onDelete,
  onTest,
}: HostListProps): React.JSX.Element {
  const { token } = theme.useToken();

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100%",
        borderRight: `1px solid ${token.colorBorderSecondary}`,
      }}
    >
      {/* 标题 + 添加按钮 */}
      <div
        style={{
          padding: "8px 8px 4px",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
        }}
      >
        <span
          style={{
            fontSize: 12,
            fontWeight: 600,
            color: token.colorTextSecondary,
          }}
        >
          主机 ({hosts.length})
        </span>
        <Button
          type="text"
          size="small"
          icon={<PlusOutlined />}
          onClick={onAdd}
          title="添加主机"
        />
      </div>

      {/* 主机列表 */}
      <div style={{ flex: 1, overflow: "auto" }}>
        <List
          size="small"
          dataSource={hosts}
          renderItem={(host) => {
            const selected = host.id === selectedId;
            return (
              <List.Item
                onClick={() => onSelect(host)}
                style={{
                  cursor: "pointer",
                  padding: "8px 12px",
                  background: selected ? token.colorPrimaryBg : "transparent",
                  borderLeft: selected
                    ? `3px solid ${token.colorPrimary}`
                    : "3px solid transparent",
                }}
              >
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    width: "100%",
                    gap: 8,
                  }}
                >
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div
                      style={{
                        fontSize: 13,
                        fontWeight: selected ? 600 : 400,
                        color: token.colorText,
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                      }}
                    >
                      {host.name}
                    </div>
                    <div
                      style={{
                        fontSize: 11,
                        color: token.colorTextSecondary,
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                      }}
                    >
                      {host.user}@{host.addr}:{host.port}
                    </div>
                  </div>
                  <Tooltip title="测试连接">
                    <Button
                      type="text"
                      size="small"
                      icon={<LinkOutlined />}
                      onClick={(e) => {
                        e.stopPropagation();
                        onTest(host);
                      }}
                    />
                  </Tooltip>
                  <Popconfirm
                    title="确定删除该主机？"
                    onConfirm={(e) => {
                      e?.stopPropagation();
                      onDelete(host.id);
                    }}
                    okText="删除"
                    cancelText="取消"
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
              </List.Item>
            );
          }}
        />
      </div>
    </div>
  );
}
