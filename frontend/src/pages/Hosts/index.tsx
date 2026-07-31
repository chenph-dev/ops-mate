import { useState, useCallback } from "react";
import { message } from "antd";
import type { hoststore } from "@wailsjs/go/models";

type HostMeta = hoststore.HostMeta;
type HostInput = hoststore.HostInput;
import { useHosts } from "@/hooks/useHosts";
import { useSessions } from "@/hooks/useSessions";
import { useWailsEvents } from "@/hooks/useWailsEvents";
import HostList from "@/components/HostList";
import HostForm from "@/components/HostForm";
import Terminal, { type TerminalLine } from "@/components/Terminal";
import AIChat from "@/components/AIChat";

export default function HostsPage(): React.JSX.Element {
  const { hosts, loading, addHost, removeHost, testConnection } = useHosts();
  const [selectedHost, setSelectedHost] = useState<HostMeta | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [terminalLines, setTerminalLines] = useState<TerminalLine[]>([]);

  const sessions = useSessions(selectedHost?.id ?? null);

  // Wails 事件处理
  const onCommand = useCallback(
    (event: { data: unknown }) => {
      if (event.data && typeof event.data === "object") {
        const d = event.data as Record<string, unknown>;
        if ("command" in d) {
          sessions.setPendingCommand(
            d as unknown as Parameters<
              typeof sessions.handleEvent
            >[0]["data"] & {
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
    setTerminalLines((prev) => [
      ...prev,
      { stream: "info", text: `状态: ${event.data}` },
    ]);
  }, []);

  useWailsEvents(onCommand, onState);

  const handleTest = useCallback(async (host: HostMeta) => {
    const input: HostInput = {
      name: host.name,
      addr: host.addr,
      port: host.port,
      user: host.user,
      authType: host.authType,
      secret: "", // 测试需要完整凭据，这里仅提示
    };
    message.info("请编辑主机以测试连接（需要密码/密钥）");
  }, []);

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "220px 1fr 1.2fr",
        height: "100%",
        gap: 0,
      }}
    >
      {/* 左：主机列表 */}
      <HostList
        hosts={hosts}
        selectedId={selectedHost?.id ?? null}
        onSelect={setSelectedHost}
        onAdd={() => setFormOpen(true)}
        onDelete={removeHost}
        onTest={handleTest}
      />

      {/* 中：终端输出 */}
      <Terminal lines={terminalLines} onClear={() => setTerminalLines([])} />

      {/* 右：AI 对话 */}
      {selectedHost ? (
        <AIChat
          conversations={sessions.conversations}
          activeSession={sessions.activeSession}
          messages={sessions.messages}
          pendingCommand={sessions.pendingCommand}
          sessionState={sessions.sessionState}
          hostName={selectedHost.name}
          onSelectSession={sessions.selectSession}
          onCreateSession={sessions.createSession}
          onDeleteSession={sessions.removeConversation}
          onSendMessage={sessions.sendMessage}
          onApprove={sessions.approve}
          onReject={sessions.reject}
        />
      ) : (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "var(--ant-color-text-secondary)",
            fontSize: 13,
          }}
        >
          请选择一个主机开始对话
        </div>
      )}

      {/* 主机表单弹窗 */}
      <HostForm
        open={formOpen}
        onCancel={() => setFormOpen(false)}
        onSubmit={async (input) => {
          await addHost(input);
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
