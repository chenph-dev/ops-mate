import { useState } from "react";
import { App as AntdApp, Button, Empty, Input, Spin } from "antd";
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
} from "@ant-design/icons";
import { useSftp } from "@/hooks/useSftp";
import type { sftp } from "@wailsjs/go/models";

function formatSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(1)} GB`;
}

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
  // 选中的条目完整路径（单击选中，再次点击取消）
  const [selected, setSelected] = useState<string | null>(null);

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
        <Button size="small" icon={<UploadOutlined />} onClick={() => void sf.upload(sf.path)}>
          上传
        </Button>
        <Button
          size="small"
          icon={<DownloadOutlined />}
          disabled={!selected}
          onClick={() => selected && void sf.download(selected)}
        >
          下载
        </Button>
        <Button size="small" icon={<EditOutlined />} disabled={!selected} onClick={handleRename}>
          重命名
        </Button>
        <Button size="small" danger icon={<DeleteOutlined />} disabled={!selected} onClick={handleDelete}>
          删除
        </Button>
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
          sf.entries.map((e: sftp.Entry) => {
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
                {!e.isDir && (
                  <span
                    style={{
                      color: "var(--antd-color-text-secondary)",
                      fontSize: 13,
                      width: 72,
                      textAlign: "right",
                    }}
                  >
                    {formatSize(e.size)}
                  </span>
                )}
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
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
