import { useEffect, useMemo, useState } from "react";
import { App as AntdApp, Button, Empty, Input, Popover, Progress, Spin, Table, Tag, Tabs, Tooltip } from "antd";
import type { ColumnsType } from "antd/es/table";
import { EventsOn } from "@wailsjs/runtime/runtime";
import {
  ArrowUpOutlined,
  ReloadOutlined,
  FolderAddOutlined,
  UploadOutlined,
  DownloadOutlined,
  DeleteOutlined,
  EditOutlined,
  FolderOutlined,
  FileOutlined,
  BarsOutlined,
  PauseOutlined,
  CaretRightOutlined,
  StopOutlined,
} from "@ant-design/icons";
import { useSftp } from "@/hooks/useSftp";
import type { sftp } from "@wailsjs/go/models";

const TASK_STATUS_META: Record<string, { text: string; color: string }> = {
  queued: { text: "队列中", color: "default" },
  running: { text: "传输中", color: "processing" },
  paused: { text: "已暂停", color: "warning" },
  done: { text: "完成", color: "success" },
  error: { text: "失败", color: "error" },
  cancelled: { text: "已删除", color: "default" },
};

type TaskTab = "running" | "queued" | "paused" | "cancelled" | "done";

function formatSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(1)} GB`;
}

type SortKey = "name" | "modTime" | "type" | "size";

/** 主机 SFTP 文件浏览面板（单栏远端文件管理器）。 */
export default function SftpPanel({
  hostId,
  hostName,
}: {
  hostId: string | null;
  hostName: string;
}): React.JSX.Element {
  const { modal, message } = AntdApp.useApp();
  const sf = useSftp(hostId);
  const { loadTasks } = sf;
  // 选中的条目完整路径（单击选中，再次点击取消）
  const [selected, setSelected] = useState<string | null>(null);
  const [taskOpen, setTaskOpen] = useState(false);
  const [taskTab, setTaskTab] = useState<TaskTab>("running");

  // 新任务开始时自动打开传输任务弹窗
  useEffect(() => {
    const off = EventsOn('sftp:task-start', () => {
      setTaskOpen(true);
      void loadTasks();
    });
    return off;
  }, [loadTasks]);

  // 各页签任务数量与可见列表
  const countByStatus = (s: string): number =>
    sf.tasks.filter((t) => t.status === s).length;
  const countByDone = (): number =>
    sf.tasks.filter((t) => t.status === "done" || t.status === "error").length;
  const visibleTasks = sf.tasks.filter((t) =>
    taskTab === "done"
      ? t.status === "done" || t.status === "error"
      : t.status === taskTab,
  );
  const [sort, setSort] = useState<{ key: SortKey; order: "asc" | "desc" }>({
    key: "name",
    order: "asc",
  });

  const handleSort = (key: SortKey): void => {
    setSort((prev) =>
      prev.key === key
        ? { key, order: prev.order === "asc" ? "desc" : "asc" }
        : { key, order: "asc" },
    );
  };

  // 目录固定在前，组内按排序列排序
  const sorted = useMemo(() => {
    const dirRank = (e: sftp.Entry): number => (e.isDir ? 0 : 1);
    const cmp = (a: sftp.Entry, b: sftp.Entry): number => {
      switch (sort.key) {
        case "name":
          return a.name.localeCompare(b.name);
        case "modTime":
          return a.modTime - b.modTime;
        case "type":
          return dirRank(a) - dirRank(b);
        case "size":
          return a.size - b.size;
        default:
          return 0;
      }
    };
    const arr = [...sf.entries];
    arr.sort((a, b) => {
      const d = dirRank(a) - dirRank(b);
      if (d !== 0) return d;
      return sort.order === "asc" ? cmp(a, b) : -cmp(a, b);
    });
    return arr;
  }, [sf.entries, sort]);

  const joinPath = (dir: string, name: string): string =>
    dir.endsWith("/") ? dir + name : dir + "/" + name;

  const askName = (
    title: string,
    placeholder: string,
    onOk: (name: string) => void,
  ): void => {
    let name = "";
    modal.confirm({
      title,
      content: (
        <Input
          autoFocus
          placeholder={placeholder}
          onChange={(e) => (name = e.target.value)}
        />
      ),
      onOk: () => {
        if (!name.trim()) return;
        onOk(name.trim());
      },
    });
  };

  const handleMkdir = (): void => {
    askName("新建目录", "输入目录名", (name) => {
      void sf.mkdir(joinPath(sf.path, name)).catch((e) =>
        message.error(`新建失败: ${e}`),
      );
    });
  };

  const handleRename = (): void => {
    if (!selected) {
      message.info("请先选择文件/目录");
      return;
    }
    const base = selected.split("/").pop() ?? "";
    askName("重命名", "输入新名称", (name) => {
      const parent = selected.slice(0, selected.length - base.length);
      void sf.rename(selected, parent + name).catch((e) =>
        message.error(`重命名失败: ${e}`),
      );
    });
  };

  const handleDelete = (): void => {
    if (!selected) {
      message.info("请先选择文件/目录");
      return;
    }
    modal.confirm({
      title: "删除该条目？",
      content: selected,
      okButtonProps: { danger: true },
      onOk: () => sf.remove(selected),
    });
  };

  // 传输任务表格列定义
  const columns: ColumnsType<sftp.TaskInfo> = [
    {
      title: "名称",
      key: "name",
      ellipsis: true,
      render: (_, t) => {
        // 显示远端文件名（传输的目标/源文件）
        const name = t.remotePath.split(/[\\/]/).pop();
        return (
          <span style={{ display: "flex", alignItems: "center", gap: 6 }}>
            {t.direction === "upload" ? (
              <UploadOutlined style={{ color: "var(--antd-color-primary)" }} />
            ) : (
              <DownloadOutlined style={{ color: "var(--antd-color-primary)" }} />
            )}
            <span
              title={`${t.direction === "upload" ? "上传" : "下载"} ${t.remotePath}`}
              style={{
                fontSize: 12,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {name}
            </span>
          </span>
        );
      },
    },
    {
      title: "进度",
      key: "progress",
      width: 120,
      render: (_, t) => (
        <Progress
          percent={t.total > 0 ? Math.round((t.done / t.total) * 100) : 0}
          size="small"
        />
      ),
    },
    {
      title: "状态",
      key: "status",
      width: 70,
      render: (_, t) => {
        const meta =
          TASK_STATUS_META[t.status] ?? { text: t.status, color: "default" };
        return (
          <Tag color={meta.color} style={{ margin: 0 }}>
            {meta.text}
          </Tag>
        );
      },
    },
    {
      title: "大小",
      key: "size",
      width: 110,
      render: (_, t) => (
        <span
          style={{
            fontSize: 11,
            color: "var(--antd-color-text-secondary)",
            whiteSpace: "nowrap",
          }}
        >
          {formatSize(t.done)} / {formatSize(t.total)}
        </span>
      ),
    },
    {
      title: "操作",
      key: "action",
      width: 90,
      render: (_, t) => (
        <div style={{ display: "flex", gap: 2 }}>
          {t.status === "running" && (
            <Tooltip title="暂停">
              <Button
                size="small"
                type="text"
                icon={<PauseOutlined />}
                onClick={() => void sf.pauseTask(t.id)}
              />
            </Tooltip>
          )}
          {t.status === "paused" && (
            <Tooltip title="继续">
              <Button
                size="small"
                type="text"
                icon={<CaretRightOutlined />}
                onClick={() => void sf.resumeTask(t.id)}
              />
            </Tooltip>
          )}
          {(t.status === "running" ||
            t.status === "paused" ||
            t.status === "queued") && (
            <Tooltip title="取消">
              <Button
                size="small"
                type="text"
                danger
                icon={<StopOutlined />}
                onClick={() => void sf.cancelTask(t.id)}
              />
            </Tooltip>
          )}
          {(t.status === "done" ||
            t.status === "error" ||
            t.status === "cancelled") && (
            <Tooltip title="删除">
              <Button
                size="small"
                type="text"
                danger
                icon={<DeleteOutlined />}
                onClick={() => void sf.removeTask(t.id)}
              />
            </Tooltip>
          )}
        </div>
      ),
    },
  ];

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100%",
        background: "var(--antd-color-bg-elevated)",
        border: "1px solid var(--antd-color-border-secondary)",
        borderRadius: 8,
        overflow: "hidden",
      }}
    >
      {/* 顶部：路径 + 操作按钮 */}
      <div
        style={{
          padding: "6px 10px",
          display: "flex",
          alignItems: "center",
          gap: 6,
          borderBottom: "1px solid var(--antd-color-border-secondary)",
          flexShrink: 0,
        }}
      >
        <span style={{ fontSize: 13, fontWeight: 600 }}>SFTP · {hostName}</span>
        <span
          style={{
            fontSize: 12,
            color: "var(--antd-color-text-secondary)",
            fontFamily: 'monospace',
            flex: 1,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {sf.path}
        </span>
        <Button size="small" icon={<ArrowUpOutlined />} title="上级目录" onClick={sf.goParent} />
        <Button size="small" icon={<ReloadOutlined />} title="刷新" onClick={() => void sf.refresh(sf.path)} />
        <Button size="small" icon={<FolderAddOutlined />} onClick={handleMkdir}>
          新建目录
        </Button>
        <Button size="small" icon={<UploadOutlined />} onClick={() => void sf.startUpload(sf.path)}>
          上传
        </Button>
        <Button
          size="small"
          icon={<DownloadOutlined />}
          disabled={!selected}
          onClick={() => selected && void sf.startDownload(selected)}
        >
          下载
        </Button>
        <Button size="small" icon={<EditOutlined />} disabled={!selected} onClick={handleRename}>
          重命名
        </Button>
        <Button size="small" danger icon={<DeleteOutlined />} disabled={!selected} onClick={handleDelete}>
          删除
        </Button>
        <Popover
          trigger="click"
          open={taskOpen}
          onOpenChange={(o) => {
            setTaskOpen(o);
            if (o) void sf.loadTasks();
          }}
          placement="bottomRight"
          content={
            <div style={{ width: 540 }}>
              <Tabs
                className="sftp-task-tabs"
                size="small"
                centered
                activeKey={taskTab}
                onChange={(k) => setTaskTab(k as TaskTab)}
                tabBarStyle={{ marginBottom: 4 }}
                items={[
                  { key: "running", label: `进行中(${countByStatus("running")})` },
                  { key: "queued", label: `队列中(${countByStatus("queued")})` },
                  { key: "paused", label: `已暂停(${countByStatus("paused")})` },
                  { key: "cancelled", label: `已删除(${countByStatus("cancelled")})` },
                  { key: "done", label: `已完成(${countByDone()})` },
                ]}
              />
              <Table
                className="sftp-task-table"
                size="small"
                rowKey="id"
                columns={columns}
                dataSource={visibleTasks}
                pagination={false}
                scroll={{ y: 360 }}
                locale={{ emptyText: "无任务" }}
              />
            </div>
          }
        >
          <Button size="small" icon={<BarsOutlined />}>
            传输任务
          </Button>
        </Popover>
      </div>

      {/* 表头：四列，点击排序 */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          padding: "4px 6px",
          borderBottom: "1px solid var(--antd-color-border-secondary)",
          fontSize: 12,
          fontWeight: 600,
          color: "var(--antd-color-text-secondary)",
          flexShrink: 0,
        }}
      >
        <span style={{ flex: 1, cursor: "pointer" }} onClick={() => handleSort("name")}>
          名称 {sort.key === "name" && (sort.order === "asc" ? "↑" : "↓")}
        </span>
        <span
          style={{ width: 126, textAlign: "right", cursor: "pointer" }}
          onClick={() => handleSort("modTime")}
        >
          修改时间 {sort.key === "modTime" && (sort.order === "asc" ? "↑" : "↓")}
        </span>
        <span
          style={{ width: 64, textAlign: "center", cursor: "pointer" }}
          onClick={() => handleSort("type")}
        >
          类型 {sort.key === "type" && (sort.order === "asc" ? "↑" : "↓")}
        </span>
        <span
          style={{ width: 72, textAlign: "right", cursor: "pointer" }}
          onClick={() => handleSort("size")}
        >
          大小 {sort.key === "size" && (sort.order === "asc" ? "↑" : "↓")}
        </span>
      </div>

      {/* 文件列表 */}
      <div style={{ flex: 1, overflow: "auto", padding: "4px 8px" }}>
        {sf.loading ? (
          <div style={{ display: "flex", justifyContent: "center", padding: 24 }}>
            <Spin size="small" />
          </div>
        ) : sf.error ? (
          <Empty description={sf.error} />
        ) : sf.entries.length === 0 ? (
          <Empty description="空目录" />
        ) : (
          sorted.map((e: sftp.Entry) => {
            const full = joinPath(sf.path, e.name);
            const isSel = selected === full;
            return (
              <div
                key={full}
                onClick={() => setSelected(isSel ? null : full)}
                onDoubleClick={() => {
                  if (e.isDir) void sf.refresh(full);
                }}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                  padding: "5px 6px",
                  borderRadius: 4,
                  cursor: "pointer",
                  background: isSel
                    ? "var(--antd-color-primary-bg)"
                    : undefined,
                  fontSize: 14,
                }}
              >
                {e.isDir ? (
                  <FolderOutlined style={{ color: "var(--antd-color-primary)" }} />
                ) : (
                  <FileOutlined style={{ color: "var(--antd-color-text-tertiary)" }} />
                )}
                <span
                  style={{
                    flex: 1,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {e.name}
                </span>
                <span
                  style={{
                    color: "var(--antd-color-text-tertiary)",
                    fontSize: 13,
                    width: 126,
                    textAlign: "right",
                  }}
                >
                  {new Date(e.modTime * 1000).toLocaleString()}
                </span>
                <span
                  style={{
                    color: "var(--antd-color-text-secondary)",
                    fontSize: 13,
                    width: 64,
                    textAlign: "center",
                  }}
                >
                  {e.isDir ? "目录" : "文件"}
                </span>
                <span
                  style={{
                    color: "var(--antd-color-text-secondary)",
                    fontSize: 13,
                    width: 72,
                    textAlign: "right",
                  }}
                >
                  {e.isDir ? "" : formatSize(e.size)}
                </span>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
