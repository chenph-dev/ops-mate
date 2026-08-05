import { useState, useCallback, useEffect, useRef } from 'react';
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
  const [sidebarWidth, setSidebarWidth] = useState<number>(() => {
    const saved = Number(localStorage.getItem('hosts-sidebar-width'));
    return saved >= 160 && saved <= 480 ? saved : 200;
  });
  const [sidebarResizeHover, setSidebarResizeHover] = useState(false);
  const sidebarResizeRef = useRef<{ startX: number; startW: number } | null>(null);

  // 分隔条左右拖动调整左侧主机列表宽度，实时持久化。
  const onSidebarResize = useCallback(
    (e: React.MouseEvent<HTMLDivElement>): void => {
      e.preventDefault();
      sidebarResizeRef.current = { startX: e.clientX, startW: sidebarWidth };
      const onMove = (ev: MouseEvent): void => {
        const r = sidebarResizeRef.current;
        if (!r) return;
        const next = Math.min(
          Math.max(r.startW + (ev.clientX - r.startX), 160),
          480,
        );
        setSidebarWidth(next);
        localStorage.setItem('hosts-sidebar-width', String(next));
      };
      const onUp = (): void => {
        sidebarResizeRef.current = null;
        window.removeEventListener('mousemove', onMove);
        window.removeEventListener('mouseup', onUp);
      };
      window.addEventListener('mousemove', onMove);
      window.addEventListener('mouseup', onUp);
    },
    [sidebarWidth],
  );

  const sessions = useSessions(selectedHost?.id ?? null);
  const terminal = useTerminal(selectedHost?.id ?? null);

  // 审批卡「在终端执行」：把 AI 提议的命令直接发到右侧终端并回车执行。
  const runInTerminal = useCallback(
    (cmd: string): void => {
      if (!terminal.connected) {
        message.info('终端未连接，请先双击主机打开终端');
        return;
      }
      terminal.sendData(cmd + '\r');
    },
    // terminal 对象每次 render 重建，但 connected/sendData 各自稳定，逐项声明依赖
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [terminal.connected, terminal.sendData, message],
  );

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
        // 左侧主机列表固定宽度 + 分隔条 + 右侧自适应
        gridTemplateColumns: `${sidebarWidth}px 5px minmax(0, 1fr)`,
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

      {/* 分隔条：左右拖动调整左侧宽度 */}
      <div
        onMouseDown={onSidebarResize}
        onMouseEnter={() => setSidebarResizeHover(true)}
        onMouseLeave={() => setSidebarResizeHover(false)}
        title="拖动调整宽度"
        style={{
          cursor: 'col-resize',
          background: sidebarResizeHover
            ? 'var(--antd-color-primary)'
            : 'transparent',
          opacity: sidebarResizeHover ? 0.6 : 1,
          transition: 'background 0.2s',
        }}
      />

      {/* 右：终端 + AI 面板（并排，AI 面板固定宽度、终端自适应让出空间） */}
      <div
        style={{
          position: 'relative',
          height: '100%',
          display: 'flex',
          minWidth: 0,
        }}
      >
        {/* 终端区域：占据 AI 面板之外的剩余宽度 */}
        <div style={{ flex: 1, minWidth: 0 }}>
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
            aiOpen={!aiCollapsed}
            onToggleAI={() => setAiCollapsed(!aiCollapsed)}
          />
        </div>

        {/* AI 面板：收起时渲染右下角按钮（不占位），展开时并排固定宽度 */}
        <AIPanel
          activeSession={sessions.activeSession}
          messages={sessions.messages}
          conversations={sessions.conversations}
          streamingText={sessions.streamingText}
          pendingCommand={sessions.pendingCommand}
          commandStatus={sessions.commandStatus}
          sessionState={sessions.sessionState}
          lastError={sessions.lastError}
          runningCommand={sessions.runningCommand}
          runElapsed={sessions.runElapsed}
          hostName={selectedHost?.name ?? ''}
          collapsed={aiCollapsed}
          onRefreshConversations={sessions.refreshConversations}
          onSwitchConversation={sessions.switchConversation}
          onDeleteConversation={sessions.deleteConversation}
          onRenameConversation={sessions.renameConversation}
          onToggleCollapse={() => setAiCollapsed(!aiCollapsed)}
          onSendMessage={sessions.sendMessage}
          onClearMessages={sessions.clearMessages}
          onRunInTerminal={runInTerminal}
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
