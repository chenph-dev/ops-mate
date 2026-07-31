import { useCallback, useEffect, useState } from 'react';
import { ListHosts, SaveHost, DeleteHost, TestConnection } from '@wailsjs/go/handler/HostsHandler';
import type { hoststore } from '@wailsjs/go/models';

type HostMeta = hoststore.HostMeta;
type HostInput = hoststore.HostInput;

export function useHosts() {
  const [hosts, setHosts] = useState<HostMeta[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const list = await ListHosts();
      setHosts(list);
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
    await DeleteHost(id);
    await refresh();
  }, [refresh]);

  const testConnection = useCallback(async (input: HostInput) => {
    return TestConnection(input);
  }, []);

  return { hosts, loading, refresh, addHost, removeHost, testConnection };
}
