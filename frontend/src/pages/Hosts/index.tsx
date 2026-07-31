import { useState, useCallback } from 'react';
import { message, Modal, Input } from 'antd';
import type { hoststore } from '@wailsjs/go/models';

type TreeNode = hoststore.TreeNode;
type HostInput = hoststore.HostInput;
import { useHosts } from '@/hooks/useHosts';
import { useSessions } from '@/hooks/useSessions';
import { useWailsEvents } from '@/hooks/useWailsEvents';
import { useThemeToggle } from '@/context/ThemeContext';
import { useTerminal, type TerminalEntry } from '@/hooks/useTerminal';
import HostList from '@/components/HostList';
import HostForm from '@/components/HostForm';
import Terminal from '@/components/Terminal';
import AIPanel from '@/components/AIPanel';

export default function HostsPage(): React.JSX.Element {
  const { isDark } = useThemeToggle();
  const { tree, addHost, removeHost, testConnection, createFolder, deleteNode } = useHosts();
  const [selectedHost, setSelectedHost] = useState<TreeNode | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [formParentId, setFormParentId] = useState('');
  const [aiCollapsed, setAiCollapsed] = useState(true);

  const sessions = useSessions(selectedHost?.id ?? null);
  const terminal = useTerminal(selectedHost?.id ?? null);

  // Wails 事件处理
  const onCommand = useCallback(
    (event: { data: unknown }) => {
      if (event.data && typeof event.data === 'object') {
        const d = event.data as Record<string, unknown>;
        if ('command' in d) {
          sessions.setPendingCommand(
            d as unknown as Parameters<
              typeof sessions.handleEvent
            >[0]['data'] & {
              command: string;
              why: string;
              risk: string;
              assessedRisk: string;
            },
          );
        }
      }
    },
    [sessions],
  );

  const onState = useCallback((event: { data: unknown }) => {
    // 状态变化可通过 terminal hook 处理
  }, []);

  useWailsEvents(onCommand, onState);

  const handleAddFolder = useCallback((parentId: string) => {
    let name = '';
    Modal.confirm({
      title: '新建目录',
      content: (
        <Input
          autoFocus
          placeholder="输入目录名称"
          onChange={(e) => (name = e.target.value)}
        />
      ),
      onOk: async () => {
        if (name.trim()) {
          await createFolder(name.trim(), parentId);
        }
      },
    });
  }, [createFolder]);

  const handleAddHost = useCallback((parentId: string) => {
    setFormParentId(parentId);
    setFormOpen(true);
  }, []);

  const handleEditHost = useCallback((host: TreeNode) => {
    message.info('编辑功能待实现');
  }, []);

  const handleDelete = useCallback(async (node: TreeNode) => {
    Modal.confirm({
      title: `确定删除"${node.name}"？`,
      content: node.nodeType === 'folder' ? '目录内的所有主机将被一并删除。' : '',
      onOk: () => deleteNode(node.id),
    });
  }, [deleteNode]);

  const handleTest = useCallback(async (host: TreeNode) => {
    message.info('请编辑主机以测试连接（需要密码/密钥）');
  }, []);

  const handleSelect = useCallback((node: TreeNode) => {
    setSelectedHost(node);
    terminal.clearEntries();
  }, [terminal]);

  const handleTerminalCommand = useCallback((command: string) => {
    terminal.runCommand(command);
  }, [terminal]);

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '240px 1fr',
        height: '100%',
        gap: 0,
      }}
    >
      {/* 左：主机树 */}
      <HostList
        treeData={tree}
        selectedId={selectedHost?.id ?? null}
        onSelect={handleSelect}
        onAddHost={handleAddHost}
        onAddFolder={handleAddFolder}
        onEditHost={handleEditHost}
        onDelete={handleDelete}
        onTest={handleTest}
      />

      {/* 右：终端 + AI 面板（悬浮） */}
      <div style={{ position: 'relative', height: '100%' }}>
        {/* 终端区域：占满整个右侧 */}
        <Terminal
          entries={terminal.entries}
          isDark={isDark}
          interactive={true}
          hostConnected={!!selectedHost}
          onCommand={handleTerminalCommand}
        />

        {/* AI 面板：悬浮在终端之上 */}
        <AIPanel
          messages={sessions.messages}
          pendingCommand={sessions.pendingCommand}
          sessionState={sessions.sessionState}
          hostName={selectedHost?.name ?? ''}
          collapsed={aiCollapsed}
          onToggleCollapse={() => setAiCollapsed(!aiCollapsed)}
          onSendMessage={sessions.sendMessage}
          onApprove={sessions.approve}
          onReject={sessions.reject}
        />
      </div>

      {/* 主机表单弹窗 */}
      <HostForm
        open={formOpen}
        onCancel={() => setFormOpen(false)}
        onSubmit={async (input) => {
          await addHost({ ...input, parentId: formParentId });
          setFormOpen(false);
        }}
        onTest={async (input) => {
          const ok = await testConnection(input);
          return ok;
        }}
      />
    </div>
  );
}
