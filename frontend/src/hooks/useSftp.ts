import { useCallback, useEffect, useState } from 'react';
import {
  ListSftpDir,
  SftpMkdir,
  SftpDelete,
  SftpRename,
  StartUpload,
  StartDownload,
  PauseTask,
  ResumeTask,
  CancelTask,
  RemoveTask,
  ListTasks,
} from '@wailsjs/go/sftp/SftpHandler';
import { EventsOn } from '@wailsjs/runtime/runtime';
import i18n from '@/i18n';
import type { sftp } from '@wailsjs/go/models';

type SftpTask = sftp.TaskInfo;

/**
 * 资产 SFTP 文件浏览状态与传输任务。
 * 当前路径 path 为远端绝对路径；操作成功后自动刷新当前目录。
 */
export function useSftp(hostId: string | null): {
  path: string;
  entries: sftp.Entry[];
  loading: boolean;
  error: string | null;
  refresh: (dir: string) => Promise<void>;
  goParent: () => void;
  mkdir: (dir: string) => Promise<void>;
  remove: (p: string) => Promise<void>;
  rename: (oldPath: string, newPath: string) => Promise<void>;
  startUpload: (remoteDir: string) => Promise<void>;
  startDownload: (remotePath: string) => Promise<void>;
  tasks: SftpTask[];
  loadTasks: () => Promise<void>;
  pauseTask: (id: string) => Promise<void>;
  resumeTask: (id: string) => Promise<void>;
  cancelTask: (id: string) => Promise<void>;
  removeTask: (id: string) => Promise<void>;
} {
  const [path, setPath] = useState('/');
  const [entries, setEntries] = useState<sftp.Entry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [tasks, setTasks] = useState<SftpTask[]>([]);

  const refresh = useCallback(
    async (dir: string) => {
      if (!hostId) return;
      setLoading(true);
      try {
        const list = await ListSftpDir(hostId, dir);
        setEntries(list ?? []);
        setPath(dir);
        setError(null);
      } catch (e) {
        setError(typeof e === 'string' ? e : i18n.t('sftp:listFailed'));
      } finally {
        setLoading(false);
      }
    },
    [hostId],
  );

  // 资产变化时进入根目录（异步加载，setState 在 await 之后，避免 effect 同步 setState）
  useEffect(() => {
    if (!hostId) return;
    let alive = true;
    void (async () => {
      try {
        const list = await ListSftpDir(hostId, '/');
        if (!alive) return;
        setEntries(list ?? []);
        setPath('/');
        setError(null);
      } catch (e) {
        if (!alive) return;
        setError(typeof e === 'string' ? e : i18n.t('sftp:listFailed'));
      }
    })();
    return () => {
      alive = false;
    };
  }, [hostId]);

  const loadTasks = useCallback(async (): Promise<void> => {
    try {
      const list = await ListTasks();
      setTasks(list ?? []);
    } catch {
      // 任务列表拉取失败不阻断
    }
  }, []);

  // 订阅传输进度事件，实时更新任务进度。
  // 注意：sftp:progress 载荷是顶层 map{taskID,done,total,status}（无 data 包装，
  // 与 agent 事件的 {sessionId,data} 不同），故直接读 raw。
  useEffect(() => {
    const off = EventsOn(
      'sftp:progress',
      (raw: {
        taskID?: string;
        done?: number;
        total?: number;
        status?: string;
      }) => {
        const d = raw;
        if (!d?.taskID) return;
        setTasks((prev) =>
          prev.map((t) =>
            t.id === d.taskID
              ? {
                  ...t,
                  done: d.done ?? t.done,
                  total: d.total ?? t.total,
                  status: d.status ?? t.status,
                }
              : t,
          ),
        );
      },
    );
    return off;
  }, []);

  const goParent = useCallback((): void => {
    const trimmed = path.replace(/\/+$/, '');
    const idx = trimmed.lastIndexOf('/');
    const parent = idx <= 0 ? '/' : trimmed.slice(0, idx);
    void refresh(parent);
  }, [path, refresh]);

  const mkdir = useCallback(
    async (dir: string) => {
      if (!hostId) return;
      await SftpMkdir(hostId, dir);
      await refresh(path);
    },
    [hostId, path, refresh],
  );

  const remove = useCallback(
    async (p: string) => {
      if (!hostId) return;
      await SftpDelete(hostId, p);
      await refresh(path);
    },
    [hostId, path, refresh],
  );

  const rename = useCallback(
    async (oldPath: string, newPath: string) => {
      if (!hostId) return;
      await SftpRename(hostId, oldPath, newPath);
      await refresh(path);
    },
    [hostId, path, refresh],
  );

  /** 启动上传：后端对话框选本地文件，上传到远端目录。 */
  const startUpload = useCallback(
    async (remoteDir: string) => {
      if (!hostId) return;
      const id = await StartUpload(hostId, remoteDir);
      if (id) await loadTasks();
    },
    [hostId, loadTasks],
  );

  /** 启动下载：后端对话框选保存位置，下载远端文件。 */
  const startDownload = useCallback(
    async (remotePath: string) => {
      if (!hostId) return;
      const id = await StartDownload(hostId, remotePath);
      if (id) await loadTasks();
    },
    [hostId, loadTasks],
  );

  const pauseTask = useCallback(
    async (id: string) => {
      await PauseTask(id);
      await loadTasks();
    },
    [loadTasks],
  );

  const resumeTask = useCallback(
    async (id: string) => {
      await ResumeTask(id);
      await loadTasks();
    },
    [loadTasks],
  );

  const cancelTask = useCallback(
    async (id: string) => {
      await CancelTask(id);
      await loadTasks();
    },
    [loadTasks],
  );

  /** 移除任务记录（清理已结束/已取消任务）。 */
  const removeTask = useCallback(
    async (id: string) => {
      await RemoveTask(id);
      await loadTasks();
    },
    [loadTasks],
  );

  return {
    path,
    entries,
    loading,
    error,
    refresh,
    goParent,
    mkdir,
    remove,
    rename,
    startUpload,
    startDownload,
    tasks,
    loadTasks,
    pauseTask,
    resumeTask,
    cancelTask,
    removeTask,
  };
}
