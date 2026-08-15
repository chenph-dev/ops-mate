import { useState, useCallback, useEffect, useRef } from 'react';
import { App as AntdApp, Button, Input } from 'antd';
import { useTranslation } from 'react-i18next';
import type { hoststore } from '@wailsjs/go/models';

type TreeNode = hoststore.TreeNode;
type HostInput = hoststore.HostInput;
import { useConnectors } from '@/hooks/useConnectors';
import { useHosts } from '@/hooks/useHosts';
import { useSessions } from '@/hooks/useSessions';
import { useThemeToggle } from '@/context/ThemeContext';
import { useTerminalSessions } from '@/hooks/useTerminalSessions';
import HostList from '@/components/HostList';
import HostForm from '@/components/HostForm';
import SftpPanel from '@/components/SftpPanel';
import Terminal from '@/components/Terminal';
import AIPanel from '@/components/AIPanel';
import WinRmPanel from '@/components/WinRmPanel';
import DbPanel from '@/components/DbPanel';

const MAX_TABS = 6;

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
    autoApprove: node.autoApprove ?? 'inherit',
    protocol: node.protocol ?? 'ssh',
    rdpPort: node.rdpPort ?? 3389,
    params: node.params ?? {},
  };
}

function hostAddrOf(node: TreeNode): string {
  return `${node.user}@${node.addr}:${node.port}`;
}

export default function HostsPage(): React.JSX.Element {
  const { message, modal } = AntdApp.useApp();
  const { isDark } = useThemeToggle();
  const { t } = useTranslation('hosts');
  const { t: tc } = useTranslation('common');
  const { isDB } = useConnectors();
  const {
    tree,
    addHost,
    updateHost,
    getHostSecret,
    testConnection,
    createFolder,
    deleteNode,
  } = useHosts();
  const [selectedHost, setSelectedHost] = useState<TreeNode | null>(null);
  const [panelHost, setPanelHost] = useState<TreeNode | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [formParentId, setFormParentId] = useState('');
  // 节点编辑：非空表示编辑该资产，空表示新增
  const [editingHostId, setEditingHostId] = useState<string | null>(null);
  const [formInitialValues, setFormInitialValues] = useState<HostInput | null>(
    null,
  );
  // 右侧视图：终端+AI / SFTP
  const [view, setView] = useState<'terminal' | 'sftp'>('terminal');
  const [aiCollapsed, setAiCollapsed] = useState(true);
  const [sidebarWidth, setSidebarWidth] = useState<number>(() => {
    const saved = Number(localStorage.getItem('hosts-sidebar-width'));
    return saved >= 160 && saved <= 480 ? saved : 200;
  });
  const [sidebarResizeHover, setSidebarResizeHover] = useState(false);
  const sidebarResizeRef = useRef<{ startX: number; startW: number } | null>(
    null,
  );

  // 多标签终端会话
  const terminal = useTerminalSessions();
  // 当前激活标签及其资产（AI 面板绑定激活标签，不随标签重复）
  const activeTab =
    terminal.tabs.find((t) => t.key === terminal.activeKey) ?? null;
  const activeHostID =
    (panelHost &&
    (panelHost.protocol === 'winrm' || isDB(panelHost.protocol))
      ? panelHost.id
      : activeTab?.hostID) ?? null;
  const sessions = useSessions(activeHostID);

  // 分隔条左右拖动调整左侧资产列表宽度，实时持久化。
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

  // 审批卡「在终端执行」：把 AI 提议的命令发到当前激活标签终端并回车执行。
  // 普通函数（非 useCallback）：需读取每轮 render 最新的 terminal/activeTab，无 memo 价值。
  const runInTerminal = (cmd: string): void => {
    if (!activeTab?.connected) {
      message.info(t('terminal.notConnected'));
      return;
    }
    terminal.sendData(activeTab.key, cmd + '\r');
  };

  // latest-ref：effect 里通过最新引用取 terminal/sessions，避免依赖对象触发 exhaustive-deps
  const terminalRef = useRef(terminal);
  useEffect(() => {
    terminalRef.current = terminal;
  });
  const attachRef = useRef(sessions.attach);
  useEffect(() => {
    attachRef.current = sessions.attach;
  });

  // 页面卸载时关闭全部终端会话（terminalRef 为稳定 ref，空依赖即可）
  useEffect(() => {
    return () => {
      terminalRef.current.closeAll();
    };
  }, []);

  // 激活标签变化时接入对应资产的 AI 会话（懒创建 + 加载历史）
  useEffect(() => {
    if (activeHostID) {
      void attachRef.current();
    }
  }, [activeHostID]);

  // 双击/连接入口：按协议分流。SSH 走多标签终端，WinRM 走命令执行器面板。
  const openHost = (node: TreeNode): void => {
    setSelectedHost(node);
    if (node.protocol === 'winrm' || isDB(node.protocol)) {
      setPanelHost(node);
      setView('terminal');
      return;
    }
    setPanelHost(null);
    if (terminal.tabs.length >= MAX_TABS) {
      message.warning(t('tabs.maxWarning', { count: MAX_TABS }));
      return;
    }
    terminal.open(node.id, node.name, hostAddrOf(node));
  };

  const handleAddFolder = useCallback(
    (parentId: string) => {
      let name = '';
      modal.confirm({
        title: t('modal.newFolder'),
        content: (
          <Input
            autoFocus
            placeholder={t('modal.folderNamePlaceholder')}
            onChange={(e) => (name = e.target.value)}
          />
        ),
        onOk: async () => {
          if (name.trim()) {
            await createFolder(name.trim(), parentId);
          }
        },
      });
    },
    [createFolder, modal, t],
  );

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

  const handleDelete = useCallback(
    async (node: TreeNode) => {
      modal.confirm({
        title: t('modal.deleteTitle', { name: node.name }),
        content:
          node.nodeType === 'folder' ? t('modal.deleteFolderContent') : '',
        onOk: () => {
          void deleteNode(node.id);
          // 删除当前打开的资产时清理右侧面板/选中态，避免渲染已删除节点
          if (panelHost?.id === node.id) setPanelHost(null);
          if (selectedHost?.id === node.id) setSelectedHost(null);
        },
      });
    },
    [deleteNode, modal, t, panelHost, selectedHost],
  );

  // 右键「连接」菜单：与双击一致，新建终端标签（普通函数，依赖最新 openHost）
  const handleTest = (node: TreeNode): void => {
    openHost(node);
  };

  // 右键「SFTP 文件」：确保该资产成为激活标签后切到 SFTP 视图（SFTP 绑定激活标签资产）
  const handleSftp = (node: TreeNode): void => {
    openHost(node);
    setView('sftp');
  };

  const handleSelect = useCallback((node: TreeNode) => {
    setSelectedHost(node);
  }, []);

  // 双击资产：新建终端标签（普通函数，依赖最新 openHost）
  const handleDoubleClick = (node: TreeNode): void => {
    openHost(node);
  };

  const aiPanelProps = {
    activeSession: sessions.activeSession,
    messages: sessions.messages,
    conversations: sessions.conversations,
    streamingText: sessions.streamingText,
    pendingCommand: sessions.pendingCommand,
    commandStatus: sessions.commandStatus,
    pendingPlan: sessions.pendingPlan,
    planStatus: sessions.planStatus,
    sessionState: sessions.sessionState,
    lastError: sessions.lastError,
    runningCommand: sessions.runningCommand,
    runElapsed: sessions.runElapsed,
    runOutput: sessions.runOutput,
    collapsed: aiCollapsed,
    onRefreshConversations: sessions.refreshConversations,
    onSwitchConversation: sessions.switchConversation,
    onDeleteConversation: sessions.deleteConversation,
    onRenameConversation: sessions.renameConversation,
    onToggleCollapse: () => setAiCollapsed(!aiCollapsed),
    onSendMessage: sessions.sendMessage,
    onClearMessages: sessions.clearMessages,
    onApprove: sessions.approve,
    onReject: sessions.reject,
    onApprovePlan: sessions.approvePlan,
    onRejectPlan: sessions.rejectPlan,
    onCancel: sessions.cancel,
    onNewConversation: sessions.newConversation,
  };

  return (
    <div
      style={{
        display: 'grid',
        // 左侧资产列表固定宽度 + 分隔条 + 右侧自适应
        gridTemplateColumns: `${sidebarWidth}px 5px minmax(0, 1fr)`,
        // 行高必须确定，否则子元素 height:100% 解析不到高度，会按内容撑开（终端残留最大化高度）
        gridTemplateRows: '100%',
        height: '100%',
        gap: 0,
      }}
    >
      {/* 左：资产树 */}
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
        title={tc('dragResize')}
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
        {panelHost ? (
          <div
            style={{
              flex: 1,
              minHeight: 0,
              display: view === 'sftp' ? 'none' : 'flex',
              minWidth: 0,
            }}
          >
            {panelHost.protocol === 'winrm' ? (
              <WinRmPanel
                host={panelHost}
                aiCollapsed={aiCollapsed}
                onToggleAI={() => setAiCollapsed(!aiCollapsed)}
              />
            ) : isDB(panelHost.protocol) ? (
              <DbPanel
                host={panelHost}
                aiCollapsed={aiCollapsed}
                onToggleAI={() => setAiCollapsed(!aiCollapsed)}
              />
            ) : null}
            <AIPanel
              {...aiPanelProps}
              sshConnected={true}
              hostName={panelHost.name}
              onRunInTerminal={() => {}}
            />
          </div>
        ) : (
          <>
        {/* 标签栏：横跨整个右列顶部，终端区与 AI 面板在其下对齐（SFTP 时隐藏） */}
        <div
          style={{
            display: view === 'sftp' ? 'none' : 'flex',
            gap: 4,
            alignItems: 'center',
            padding: '4px 8px',
            borderBottom: '1px solid var(--antd-color-border-secondary)',
            background: 'var(--antd-color-bg-layout)',
            flexShrink: 0,
            overflowX: 'auto',
          }}
        >
          {terminal.tabs.length === 0 && (
            <span style={{ fontSize: 12, color: '#999' }}>
              {t('tabs.hint')}
            </span>
          )}
          {terminal.tabs.map((t) => {
            const active = t.key === terminal.activeKey;
            return (
              <div
                key={t.key}
                onClick={() => terminal.activate(t.key)}
                style={{
                  cursor: 'pointer',
                  padding: '2px 8px',
                  borderRadius: 4,
                  fontSize: 12,
                  whiteSpace: 'nowrap',
                  display: 'flex',
                  gap: 4,
                  alignItems: 'center',
                  background: active
                    ? 'var(--antd-color-primary)'
                    : 'transparent',
                  color: active ? '#fff' : 'inherit',
                  border: '1px solid var(--antd-color-border-secondary)',
                }}
              >
                <span>{t.hostName}</span>
                <span
                  onClick={(e) => {
                    e.stopPropagation();
                    terminal.closeTab(t.key);
                  }}
                  style={{ cursor: 'pointer', opacity: 0.7, fontSize: 12 }}
                >
                  ×
                </span>
              </div>
            );
          })}
        </div>

        {/* 终端视图：始终挂载（保留各标签 xterm 输出），display 控制显隐 */}
        <div
          style={{
            position: 'relative',
            flex: 1,
            minHeight: 0,
            display: view === 'sftp' ? 'none' : 'flex',
            minWidth: 0,
          }}
        >
          {/* 终端区：每个标签一个 Terminal，全部挂载，非激活隐藏 */}
          <div style={{ flex: 1, minHeight: 0, position: 'relative' }}>
            {terminal.tabs.map((t) => (
              <div
                key={t.key}
                style={{
                  position: 'absolute',
                  inset: 0,
                  display: t.key === terminal.activeKey ? 'block' : 'none',
                }}
              >
                <Terminal
                  isDark={isDark}
                  hostID={t.hostID}
                  connected={t.connected}
                  connecting={t.connecting}
                  reconnecting={t.reconnecting}
                  reconnectCount={t.reconnectCount}
                  hostName={t.hostName}
                  hostAddr={t.hostAddr}
                  onData={(d) => terminal.sendData(t.key, d)}
                  onResize={(c, r) => terminal.resize(t.key, c, r)}
                  setOutputHandler={(cb) =>
                    terminal.setOutputHandler(t.key, cb)
                  }
                  onDisconnect={() => terminal.closeTab(t.key)}
                  aiOpen={!aiCollapsed}
                  onToggleAI={() => setAiCollapsed(!aiCollapsed)}
                  onRefresh={() => terminal.refresh(t.key)}
                  onOpenSftp={() => setView('sftp')}
                />
              </div>
            ))}
          </div>

          {/* 智能体面板：绑定当前激活标签的资产；收起时渲染右下角按钮（不占位） */}
          <AIPanel
            {...aiPanelProps}
            sshConnected={activeTab?.connected ?? false}
            hostName={activeTab?.hostName ?? ''}
            onRunInTerminal={runInTerminal}
          />
        </div>
          </>
        )}

        {/* SFTP 视图：绑定当前激活标签的资产；条件挂载（切走卸载，重进重新加载根目录） */}
        {view === 'sftp' && (
          <div
            style={{
              flex: 1,
              minHeight: 0,
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            <div
              style={{
                padding: '4px 8px',
                borderBottom: '1px solid var(--antd-color-border-secondary)',
                background: 'var(--antd-color-bg-layout)',
                flexShrink: 0,
                display: 'flex',
                alignItems: 'center',
                gap: 8,
              }}
            >
              <Button size="small" onClick={() => setView('terminal')}>
                {t('view.backToTerminal')}
              </Button>
              <span style={{ fontSize: 12 }}>
                SFTP · {activeTab?.hostName ?? t('sftp.noHost')}
              </span>
            </div>
            <div style={{ flex: 1, minHeight: 0, padding: '4px 5px' }}>
              <SftpPanel
                hostId={activeTab?.hostID ?? null}
                hostName={activeTab?.hostName ?? ''}
              />
            </div>
          </div>
        )}
      </div>

      {/* 资产表单弹窗 */}
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
