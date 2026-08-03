import { useCallback, useEffect, useState } from 'react';
import {
  SaveHost,
  TestConnection,
  CreateFolder,
  ListTree,
  MoveNode,
  DeleteNode,
} from '@wailsjs/go/handler/HostsHandler';
import type { hoststore } from '@wailsjs/go/models';

type HostInput = hoststore.HostInput;
type TreeNode = hoststore.TreeNode;

export function useHosts(): {
  tree: TreeNode[];
  loading: boolean;
  refresh: () => Promise<void>;
  addHost: (input: HostInput) => Promise<string | void>;
  removeHost: (id: string) => Promise<void>;
  testConnection: (input: HostInput) => Promise<boolean>;
  createFolder: (name: string, parentId: string) => Promise<string | void>;
  moveNode: (nodeId: string, newParentId: string) => Promise<void>;
  deleteNode: (nodeId: string) => Promise<void>;
} {
  const [tree, setTree] = useState<TreeNode[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const treeData = await ListTree();
      setTree(treeData);
    } finally {
      setLoading(false);
    }
  }, []);

  // 初始加载：在 effect 中直接请求数据，不触发 loading 状态，避免同步 setState 导致级联渲染
  useEffect(() => {
    let cancelled = false;
    const load = async (): Promise<void> => {
      const treeData = await ListTree();
      if (!cancelled) {
        setTree(treeData);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  const addHost = useCallback(async (input: HostInput) => {
    const id = await SaveHost(input);
    await refresh();
    return id;
  }, [refresh]);

  const removeHost = useCallback(async (id: string) => {
    await DeleteNode(id);
    await refresh();
  }, [refresh]);

  const testConnection = useCallback(async (input: HostInput) => {
    return TestConnection(input);
  }, []);

  const createFolder = useCallback(async (name: string, parentId: string) => {
    const id = await CreateFolder(name, parentId);
    await refresh();
    return id;
  }, [refresh]);

  const moveNode = useCallback(async (nodeId: string, newParentId: string) => {
    await MoveNode(nodeId, newParentId);
    await refresh();
  }, [refresh]);

  const deleteNode = useCallback(async (nodeId: string) => {
    await DeleteNode(nodeId);
    await refresh();
  }, [refresh]);

  return { tree, loading, refresh, addHost, removeHost, testConnection, createFolder, moveNode, deleteNode };
}
