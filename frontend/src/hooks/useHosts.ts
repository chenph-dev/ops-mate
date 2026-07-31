import { useCallback, useEffect, useState } from 'react';
import {
  ListHosts,
  SaveHost,
  DeleteHost,
  TestConnection,
  CreateFolder,
  ListTree,
  MoveNode,
  DeleteNode,
} from '@wailsjs/go/handler/HostsHandler';
import type { hoststore } from '@wailsjs/go/models';

type HostMeta = hoststore.HostMeta;
type HostInput = hoststore.HostInput;
type TreeNode = hoststore.TreeNode;

export function useHosts() {
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

  useEffect(() => {
    refresh();
  }, [refresh]);

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
