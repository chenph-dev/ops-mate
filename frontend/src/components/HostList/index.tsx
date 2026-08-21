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
import { useState, useCallback, useMemo, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import type { hoststore } from '@wailsjs/go/models';
import { useConnectors } from '@/hooks/useConnectors';

type TreeNode = hoststore.TreeNode;

interface HostListProps {
  treeData: TreeNode[];
  selectedId: string | null;
  onSelect: (host: TreeNode) => void;
  onDoubleClick: (host: TreeNode) => void;
  onAddHost: (parentId: string) => void;
  onAddFolder: (parentId: string) => void;
  onEditHost: (host: TreeNode) => void;
  onEditFolder: (folder: TreeNode) => void;
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
  onEditFolder: () => void;
  onDelete: () => void;
  onTest: () => void;
  onSftp: () => void;
  showSftp: boolean;
}

function ContextMenu({
  node,
  x,
  y,
  onClose,
  onAddHost,
  onAddFolder,
  onEdit,
  onEditFolder,
  onDelete,
  onTest,
  onSftp,
  showSftp,
}: ContextMenuProps): React.JSX.Element {
  const { token } = theme.useToken();
  const { t } = useTranslation('hosts');
  const menuRef = useRef<HTMLDivElement>(null);

  // 点击菜单外部或按 ESC 关闭
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent): void => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    const handleKeyDown = (e: KeyboardEvent): void => {
      if (e.key === 'Escape') {
        onClose();
      }
    };
    // 使用 capture 确保先于其他点击处理
    document.addEventListener('mousedown', handleClickOutside, true);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside, true);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [onClose]);
  const items =
    node.nodeType === 'folder'
      ? [
          {
            key: 'add-folder',
            icon: <PlusOutlined />,
            label: t('ctx.newSubfolder'),
            onClick: onAddFolder,
          },
          {
            key: 'add-host',
            icon: <PlusOutlined />,
            label: t('ctx.newHost'),
            onClick: onAddHost,
          },
          {
            key: 'edit',
            icon: <EditOutlined />,
            label: t('ctx.edit'),
            onClick: onEditFolder,
          },
          {
            key: 'delete',
            icon: <DeleteOutlined />,
            label: t('ctx.delete'),
            danger: true,
            onClick: onDelete,
          },
        ]
      : [
          {
            key: 'connect',
            icon: <LinkOutlined />,
            label: t('ctx.connect'),
            onClick: onTest,
          },
          ...(showSftp
            ? [
                {
                  key: 'sftp',
                  icon: <FolderOpenOutlined />,
                  label: t('ctx.sftp'),
                  onClick: onSftp,
                },
              ]
            : []),
          {
            key: 'edit',
            icon: <EditOutlined />,
            label: t('ctx.edit'),
            onClick: onEdit,
          },
          {
            key: 'delete',
            icon: <DeleteOutlined />,
            label: t('ctx.delete'),
            danger: true,
            onClick: onDelete,
          },
        ];

  return (
    <div
      ref={menuRef}
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
  onEditFolder,
  onDelete,
  onTest,
  onSftp,
}: HostListProps): React.JSX.Element {
  const { token } = theme.useToken();
  const { t } = useTranslation('hosts');
  const { isDB } = useConnectors();
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
        title:
          n.nodeType === 'host' ? (
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
              {n.name}
              {n.protocol === 'winrm' ? (
                <span style={{ fontSize: 10, padding: '0 4px', borderRadius: 3, background: token.colorPrimaryBg, color: token.colorPrimary }}>WinRM</span>
              ) : isDB(n.protocol) ? (
                <span style={{ fontSize: 10, padding: '0 4px', borderRadius: 3, background: token.colorSuccessBg, color: token.colorSuccess }}>DB</span>
              ) : (
                <span style={{ fontSize: 10, padding: '0 4px', borderRadius: 3, background: token.colorFillSecondary, color: token.colorTextSecondary }}>SSH</span>
              )}
            </span>
          ) : (
            n.name
          ),
        isLeaf: n.nodeType === 'host',
        icon:
          n.nodeType === 'folder' ? <FolderOutlined /> : <DesktopOutlined />,
        children:
          n.children && n.children.length > 0 ? convert(n.children) : undefined,
        data: n,
      }));
    return convert(treeData);
  }, [treeData, token, isDB]);

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
          {t('list.title')}
        </span>
        <div style={{ display: 'flex', gap: 4 }}>
          <span
            style={{
              cursor: 'pointer',
              fontSize: 14,
              color: token.colorTextSecondary,
            }}
            title={t('modal.newFolder')}
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
              padding: '32px 16px',
              textAlign: 'center',
              color: token.colorTextSecondary,
              fontSize: 12,
            }}
          >
            <FolderOutlined style={{ fontSize: 28, display: 'block', marginBottom: 8, opacity: 0.4 }} />
            {t('list.empty')}
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
          onEditFolder={() => {
            onEditFolder(contextMenu.node);
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
          showSftp={contextMenu.node.protocol !== 'winrm' && !isDB(contextMenu.node.protocol)}
        />
      )}
    </div>
  );
}
