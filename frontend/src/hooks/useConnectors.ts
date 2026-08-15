import { useCallback, useEffect, useMemo, useState } from 'react';
import { ListDrivers } from '@wailsjs/go/connector/ConnectorHandler';
import type { connector } from '@wailsjs/go/models';

type DriverMeta = connector.DriverMeta;

// useConnectors 拉取连接类型注册表元信息（protocol 下拉、参数表单、面板路由的单一事实来源）。
export function useConnectors(): {
  drivers: DriverMeta[];
  loading: boolean;
  isDB: (protocol?: string) => boolean;
} {
  const [drivers, setDrivers] = useState<DriverMeta[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void ListDrivers().then((list) => {
      if (!cancelled) {
        setDrivers(list);
        setLoading(false);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const protocolSet = useMemo(() => new Set(drivers.map((d) => d.protocol)), [drivers]);

  const isDB = useCallback(
    (protocol?: string) =>
      (protocol ? protocolSet.has(protocol.toLowerCase()) : false),
    [protocolSet],
  );

  return { drivers, loading, isDB };
}
