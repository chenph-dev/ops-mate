import { Tree, theme } from 'antd';
import type { DataNode } from 'antd/es/tree';
import {
  DesktopOutlined,
  FolderOutlined,
  FolderOpenOutlined,
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  LinkOutlined,
} from '@ant-design/icons';
import { useState, useCallback, useMemo } from 'react';
import type { hoststore } from '@wailsjs/go/models';

type TreeNode = hoststore.TreeNode;

interface HostListProps {
  treeData: TreeNode[];
  selectedId: string | null;
  onSelect: (host: TreeNode) => void;
  onDoubleClick: (host: TreeNode) => void;
  onAddHost: (parentId: string) => void;
  onAddFolder: (parentId: string) => void;
  onEditHost: (host: TreeNode) => void;
  onDelete: (node: TreeNode) => void;
  onTest: (host: TreeNode) => void;
  onSftp: (host: TreeNode) => void;
}

interface ContextMenuProps {
  node: TreeNode;
  x: number;
  y: number;
  onClose: () => void;
  onAddHost: () => void;
  onAddFolder: () => void;
  onEdit: () => void;
  onDelete: () => void;
  onTest: () => void;
  onSftp: () => void;
}

function ContextMenu({
  node,
  x,
  y,
  onClose,
  onAddHost,
  onAddFolder,
  onEdit,
  onDelete,
  onTest,
  onSftp,
}: ContextMenuProps): React.JSX.Element {
  const { token } = theme.useToken();
  const items =
    node.nodeType === 'folder'
      ? [
          {
            key: 'add-folder',
            icon: <PlusOutlined />,
            label: '新建子目录',
            onClick: onAddFolder,
          },
          {
            key: 'add-host',
            icon: <PlusOutlined />,
            label: '新建主机',
            onClick: onAddHost,
          },
          {
            key: 'delete',
            icon: <DeleteOutlined />,
            label: '删除',
            danger: true,
            onClick: onDelete,
          },
        ]
      : [
          {
            key: 'connect',
            icon: <LinkOutlined />,
            label: '连接',
            onClick: onTest,
          },
          {
            key: 'sftp',
            icon: <FolderOpenOutlined />,
            label: 'SFTP 文件',
            onClick: onSftp,
          },
          {
            key: 'edit',
            icon: <EditOutlined />,
            label: '编辑',
            onClick: onEdit,
          },
          {
            key: 'delete',
            icon: <DeleteOutlined />,
            label: '删除',
            danger: true,
            onClick: onDelete,
          },
        ];

  return (
    <div
      style={{
        position: 'fixed',
        left: x,
        top: y,
        zIndex: 9999,
        background: token.colorBgElevated,
        borderRadius: token.borderRadiusLG,
        boxShadow: token.boxShadowSecondary,
        padding: '4px 0',
        minWidth: 140,
      }}
      onClick={onClose}
    >
      {items.map((item) => (
        <div
          key={item.key}
          style={{
            padding: '6px 16px',
            cursor: 'pointer',
            fontSize: 13,
            color: item.danger ? token.colorError : token.colorText,
          }}
          onClick={item.onClick}
        >
          {item.icon} {item.label}
        </div>
      ))}
    </div>
  );
}

export default function HostList({
  treeData,
  selectedId,
  onSelect,
  onDoubleClick,
  onAddHost,
  onAddFolder,
  onEditHost,
  onDelete,
  onTest,
  onSftp,
}: HostListProps): React.JSX.Element {
  const { token } = theme.useToken();
  const [contextMenu, setContextMenu] = useState<{
    node: TreeNode;
    x: number;
    y: number;
  } | null>(null);

  // 扩展的 DataNode 类型，携带原始数据
  interface TreeNodeData extends DataNode {
    data?: TreeNode;
  }

  // 转换为 antd Tree 的 DataNode 格式
  const treeNodes = useMemo<TreeNodeData[]>(() => {
    const convert = (nodes: TreeNode[]): TreeNodeData[] =>
      nodes.map((n) => ({
        key: n.id,
        title: n.name,
        isLeaf: n.nodeType === 'host',
        icon:
          n.nodeType === 'folder' ? <FolderOutlined /> : <DesktopOutlined />,
        children:
          n.children && n.children.length > 0 ? convert(n.children) : undefined,
        data: n,
      }));
    return convert(treeData);
  }, [treeData]);

  const handleRightClick = useCallback(
    ({ event, node }: { event: React.MouseEvent; node: TreeNodeData }) => {
      event.preventDefault();
      if (node.data) {
        setContextMenu({ node: node.data, x: event.clientX, y: event.clientY });
      }
    },
    [],
  );

  const handleSelect = useCallback(
    (_keys: React.Key[], info: { node: TreeNodeData }) => {
      if (info.node.data?.nodeType === 'host') {
        onSelect(info.node.data);
      }
    },
    [onSelect],
  );

  const handleDoubleClick = useCallback(
    (_event: React.MouseEvent, node: TreeNodeData) => {
      if (node.data?.nodeType === 'host') {
        onDoubleClick(node.data);
      }
    },
    [onDoubleClick],
  );

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        borderRight: `1px solid ${token.colorBorderSecondary}`,
      }}
    >
      {/* 标题栏 */}
      <div
        style={{
          padding: '8px 8px 4px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
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
          主机
        </span>
        <div style={{ display: 'flex', gap: 4 }}>
          <span
            style={{
              cursor: 'pointer',
              fontSize: 14,
              color: token.colorTextSecondary,
            }}
            title="新建目录"
            onClick={() => onAddFolder('')}
          >
            <PlusOutlined />
          </span>
        </div>
      </div>

      {/* 树形列表 */}
      <div style={{ flex: 1, overflow: 'auto', padding: '4px' }}>
        {treeNodes.length === 0 ? (
          <div
            style={{
              padding: 16,
              textAlign: 'center',
              color: token.colorTextSecondary,
              fontSize: 12,
            }}
          >
            右键点击此处添加目录或主机
          </div>
        ) : (
          <Tree
            showIcon
            defaultExpandAll
            selectedKeys={selectedId ? [selectedId] : []}
            treeData={treeNodes}
            onSelect={handleSelect}
            onDoubleClick={handleDoubleClick}
            onRightClick={handleRightClick}
            style={{ background: 'transparent' }}
          />
        )}
      </div>

      {/* 右键菜单 */}
      {contextMenu && (
        <ContextMenu
          node={contextMenu.node}
          x={contextMenu.x}
          y={contextMenu.y}
          onClose={() => setContextMenu(null)}
          onAddHost={() => {
            onAddHost(contextMenu.node.id);
            setContextMenu(null);
          }}
          onAddFolder={() => {
            onAddFolder(contextMenu.node.id);
            setContextMenu(null);
          }}
          onEdit={() => {
            onEditHost(contextMenu.node);
            setContextMenu(null);
          }}
          onDelete={() => {
            onDelete(contextMenu.node);
            setContextMenu(null);
          }}
          onTest={() => {
            onTest(contextMenu.node);
            setContextMenu(null);
          }}
          onSftp={() => {
            onSftp(contextMenu.node);
            setContextMenu(null);
          }}
        />
      )}
    </div>
  );
}
