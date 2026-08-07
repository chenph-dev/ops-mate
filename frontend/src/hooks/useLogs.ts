import { useCallback, useEffect, useState } from 'react';
import { ListLogs, TokenSummary } from '@wailsjs/go/handler/LogsHandler';
import type { logsstore } from '@wailsjs/go/models';

type CallLog = logsstore.CallLog;
type TokenSummary = logsstore.TokenSummary;

export function useLogs(): {
  logs: CallLog[];
  summary: TokenSummary | null;
  loading: boolean;
  refresh: () => Promise<void>;
} {
  const [logs, setLogs] = useState<CallLog[]>([]);
  const [summary, setSummary] = useState<TokenSummary | null>(null);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [logList, sum] = await Promise.all([ListLogs(200), TokenSummary()]);
      setLogs(logList);
      setSummary(sum);
    } finally {
      setLoading(false);
    }
  }, []);

  // 初始加载不触发 loading 状态，避免同步 setState 导致级联渲染
  useEffect(() => {
    let cancelled = false;
    const load = async (): Promise<void> => {
      const [logList, sum] = await Promise.all([ListLogs(200), TokenSummary()]);
      if (!cancelled) {
        setLogs(logList);
        setSummary(sum);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  return { logs, summary, loading, refresh };
}
