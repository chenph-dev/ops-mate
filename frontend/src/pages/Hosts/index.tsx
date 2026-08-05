import { useState, useCallback, useEffect } from 'react';
import { App as AntdApp, Input } from 'antd';
import type { hoststore } from '@wailsjs/go/models';

type TreeNode = hoststore.TreeNode;
import { useHosts } from '@/hooks/useHosts';
import { useSessions } from '@/hooks/useSessions';
import { useThemeToggle } from '@/context/ThemeContext';
import { useTerminal } from '@/hooks/useTerminal';
import HostList from '@/components/HostList';
import HostForm from '@/components/HostForm';
import Terminal from '@/components/Terminal';
import AIPanel from '@/components/AIPanel';

export default function HostsPage(): React.JSX.Element {
  const { message, modal } = AntdApp.useApp();
  const { isDark } = useThemeToggle();
  const { tree, addHost, testConnection, createFolder, deleteNode } = useHosts();
  const [selectedHost, setSelectedHost] = useState<TreeNode | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [formParentId, setFormParentId] = useState('');
  const [aiCollapsed, setAiCollapsed] = useState(true);

  const sessions = useSessions(selectedHost?.id ?? null);
  const terminal = useTerminal(selectedHost?.id ?? null);

  // 页面卸载时关闭终端会话（terminal.close 是稳定引用，仅卸载时执行一次）
  useEffect(() => {
    return () => {
      terminal.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [terminal.close]);

  // 选中主机时接入 AI 会话（懒创建 + 加载历史）
  useEffect(() => {
    if (selectedHost && selectedHost.nodeType === 'host') {
      void sessions.attach();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedHost?.id]);

  const handleAddFolder = useCallback((parentId: string) => {
    let name = '';
    modal.confirm({
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
  }, [createFolder, modal]);

  const handleAddHost = useCallback((parentId: string) => {
    setFormParentId(parentId);
    setFormOpen(true);
  }, []);

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const handleEditHost = useCallback((_host: TreeNode) => {
    message.info('编辑功能待实现');
  }, [message]);

  const handleDelete = useCallback(async (node: TreeNode) => {
    modal.confirm({
      title: `确定删除"${node.name}"？`,
      content: node.nodeType === 'folder' ? '目录内的所有主机将被一并删除。' : '',
      onOk: () => deleteNode(node.id),
    });
  }, [deleteNode, modal]);

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const handleTest = useCallback(async (_host: TreeNode) => {
    message.info('请编辑主机以测试连接（需要密码/密钥）');
  }, [message]);

  const handleSelect = useCallback((node: TreeNode) => {
    setSelectedHost(node);
  }, []);

  const handleDoubleClick = useCallback(async (node: TreeNode) => {
    setSelectedHost(node);
    try {
      await terminal.open(node.id);
    } catch (err) {
      message.error(`连接失败: ${err}`);
    }
  }, [terminal, message]);

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '20% 80%',
        // 行高必须确定，否则子元素 height:100% 解析不到高度，会按内容撑开（终端残留最大化高度）
        gridTemplateRows: '100%',
        height: '100%',
        gap: 0,
      }}
    >
      {/* 左：主机树 */}
      <HostList
        treeData={tree}
        selectedId={selectedHost?.id ?? null}
        onSelect={handleSelect}
        onDoubleClick={handleDoubleClick}
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
          isDark={isDark}
          hostID={selectedHost?.id}
          connected={terminal.connected}
          connecting={terminal.connecting}
          reconnecting={terminal.reconnecting}
          reconnectCount={terminal.reconnectCount}
          hostName={selectedHost?.name ?? ''}
          hostAddr={
            selectedHost
              ? `${selectedHost.user}@${selectedHost.addr}:${selectedHost.port}`
              : ''
          }
          onData={terminal.sendData}
          onResize={terminal.resize}
          setOutputHandler={terminal.setOutputHandler}
          onDisconnect={() => terminal.close()}
        />

        {/* AI 面板：悬浮在终端之上 */}
        <AIPanel
          messages={sessions.messages}
          streamingText={sessions.streamingText}
          pendingCommand={sessions.pendingCommand}
          sessionState={sessions.sessionState}
          lastError={sessions.lastError}
          hostName={selectedHost?.name ?? ''}
          collapsed={aiCollapsed}
          onToggleCollapse={() => setAiCollapsed(!aiCollapsed)}
          onSendMessage={sessions.sendMessage}
          onApprove={sessions.approve}
          onReject={sessions.reject}
          onCancel={sessions.cancel}
          onNewConversation={sessions.newConversation}
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
