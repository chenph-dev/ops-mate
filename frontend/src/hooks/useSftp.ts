import { useCallback, useEffect, useState } from 'react';
import {
  ListSftpDir,
  SftpMkdir,
  SftpDelete,
  SftpRename,
  UploadSftp,
  DownloadSftp,
} from '@wailsjs/go/handler/SftpHandler';
import type { sftp } from '@wailsjs/go/models';

/**
 * 主机 SFTP 文件浏览状态与操作。
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
  upload: (remoteDir: string) => Promise<void>;
  download: (remotePath: string) => Promise<void>;
} {
  const [path, setPath] = useState('/');
  const [entries, setEntries] = useState<sftp.Entry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
        setError(typeof e === 'string' ? e : '列目录失败');
      } finally {
        setLoading(false);
      }
    },
    [hostId],
  );

  // 主机变化时进入根目录（异步加载，setState 在 await 之后，避免 effect 同步 setState）
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
        setError(typeof e === 'string' ? e : '列目录失败');
      }
    })();
    return () => {
      alive = false;
    };
  }, [hostId]);

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

  const upload = useCallback(
    async (remoteDir: string) => {
      if (!hostId) return;
      const name = await UploadSftp(hostId, remoteDir);
      if (name) await refresh(remoteDir);
    },
    [hostId, refresh],
  );

  const download = useCallback(
    async (remotePath: string) => {
      if (!hostId) return;
      await DownloadSftp(hostId, remotePath);
    },
    [hostId],
  );

  return { path, entries, loading, error, refresh, goParent, mkdir, remove, rename, upload, download };
}
