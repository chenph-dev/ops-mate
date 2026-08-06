import { useState, useCallback, useEffect, useRef } from 'react';
import { App as AntdApp, Input, Segmented } from 'antd';
import type { hoststore } from '@wailsjs/go/models';

type TreeNode = hoststore.TreeNode;
type HostInput = hoststore.HostInput;
import { useHosts } from '@/hooks/useHosts';
import { useSessions } from '@/hooks/useSessions';
import { useThemeToggle } from '@/context/ThemeContext';
import { useTerminal } from '@/hooks/useTerminal';
import HostList from '@/components/HostList';
import HostForm from '@/components/HostForm';
import SftpPanel from '@/components/SftpPanel';
import Terminal from '@/components/Terminal';
import AIPanel from '@/components/AIPanel';

/** TreeNode → 编辑表单值：TreeNode 不含凭据，secret 留空（空则后端保留原密码）。 */
function toHostInput(node: TreeNode): HostInput {
  return {
    name: node.name,
    parentId: node.parentId ?? '',
    addr: node.addr ?? '',
    port: node.port ?? 22,
    user: node.user ?? '',
    authType: node.authType ?? 'password',
    secret: '',
  };
}

export default function HostsPage(): React.JSX.Element {
  const { message, modal } = AntdApp.useApp();
  const { isDark } = useThemeToggle();
  const { tree, addHost, updateHost, getHostSecret, testConnection, createFolder, deleteNode } = useHosts();
  const [selectedHost, setSelectedHost] = useState<TreeNode | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [formParentId, setFormParentId] = useState('');
  // 节点编辑：非空表示编辑该主机，空表示新增
  const [editingHostId, setEditingHostId] = useState<string | null>(null);
  const [formInitialValues, setFormInitialValues] = useState<HostInput | null>(null);
  // 右侧视图：终端+AI / SFTP
  const [view, setView] = useState<'terminal' | 'sftp'>('terminal');
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
    setEditingHostId(null);
    setFormInitialValues(null);
    setFormParentId(parentId);
    setFormOpen(true);
  }, []);

  const handleEditHost = useCallback(
    async (host: TreeNode) => {
      setEditingHostId(host.id);
      setFormParentId(host.parentId ?? '');
      // 加载已存密码/私钥回填（TreeNode 不含凭据），失败则留空（保留原密码）
      let secret = '';
      try {
        secret = await getHostSecret(host.id);
      } catch {
        // 忽略：留空表示不修改
      }
      setFormInitialValues({ ...toHostInput(host), secret });
      setFormOpen(true);
    },
    [getHostSecret],
  );

  const handleDelete = useCallback(async (node: TreeNode) => {
    modal.confirm({
      title: `确定删除"${node.name}"？`,
      content: node.nodeType === 'folder' ? '目录内的所有主机将被一并删除。' : '',
      onOk: () => deleteNode(node.id),
    });
  }, [deleteNode, modal]);

  // 右键「连接」菜单：与双击一致，真正打开终端
  const handleTest = useCallback(
    async (node: TreeNode) => {
      setSelectedHost(node);
      try {
        await terminal.open(node.id);
      } catch (err) {
        message.error(`连接失败: ${err}`);
      }
    },
    [terminal, message],
  );

  // 右键「SFTP 文件」：切到 SFTP 视图浏览该主机文件
  const handleSftp = useCallback((node: TreeNode) => {
    setSelectedHost(node);
    setView('sftp');
  }, []);

  // 终端工具栏「刷新连接」：重连当前选中主机。
  // terminal 对象每次 render 重建但 open 稳定，selectedHost?.id 是意图依赖。
  // eslint-disable-next-line react-hooks/preserve-manual-memoization
  const handleRefreshTerminal = useCallback(() => {
    if (selectedHost?.id) {
      void terminal.open(selectedHost.id);
    }
  }, [selectedHost?.id, terminal]);

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
        onSftp={handleSftp}
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

      {/* 右：视图切换（终端+AI / SFTP）+ 内容 */}
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          height: '100%',
          minWidth: 0,
        }}
      >
        {/* 视图切换 */}
        <div
          style={{
            padding: '4px 8px',
            borderBottom: '1px solid var(--antd-color-border-secondary)',
            background: 'var(--antd-color-bg-layout)',
            flexShrink: 0,
          }}
        >
          <Segmented
            size="small"
            value={view}
            onChange={(v) => setView(v as 'terminal' | 'sftp')}
            options={[
              { label: '终端', value: 'terminal' },
              { label: 'SFTP', value: 'sftp', disabled: !selectedHost },
            ]}
          />
        </div>
        {/* 终端视图：始终挂载（保留 xterm 输出），display 控制显隐，避免切换时重挂清空 */}
        <div
          style={{
            position: 'relative',
            flex: 1,
            minHeight: 0,
            display: view === 'sftp' ? 'none' : 'flex',
            minWidth: 0,
          }}
        >
            {/* 终端区域：占据智能体面板之外的剩余宽度 */}
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
                onRefresh={handleRefreshTerminal}
              />
            </div>

            {/* 智能体面板：收起时渲染右下角按钮（不占位），展开时并排固定宽度 */}
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
              runOutput={sessions.runOutput}
              sshConnected={terminal.connected}
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

      {/* SFTP 视图：条件挂载（切走卸载，重进重新加载根目录） */}
      {view === 'sftp' && (
        <div style={{ flex: 1, minHeight: 0, padding: '4px 5px' }}>
          <SftpPanel
            hostId={selectedHost?.id ?? null}
            hostName={selectedHost?.name ?? ''}
          />
        </div>
      )}
    </div>

      {/* 主机表单弹窗 */}
      <HostForm
        open={formOpen}
        initialValues={formInitialValues}
        onCancel={() => setFormOpen(false)}
        onSubmit={async (input) => {
          if (editingHostId) {
            await updateHost(editingHostId, input);
          } else {
            await addHost({ ...input, parentId: formParentId });
          }
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
