import { useCallback, useMemo, useState, useEffect } from 'react';
import { ListDrivers } from '@wailsjs/go/connector/ConnectorHandler';
import type { connector } from '@wailsjs/go/models';

type DriverMeta = connector.DriverMeta;

// 模块级缓存：单次拉取，所有消费者共享（避免每组件重复 IPC）。
let cachedDrivers: DriverMeta[] | null = null;
let cachedPromise: Promise<void> | null = null;

// useConnectors 拉取连接类型注册表元信息（protocol 下拉、参数表单、面板路由的单一事实来源）。
export function useConnectors(): {
  drivers: DriverMeta[];
  loading: boolean;
  isDB: (protocol?: string) => boolean;
} {
  const [drivers, setDrivers] = useState<DriverMeta[]>(cachedDrivers ?? []);
  const [loading, setLoading] = useState(cachedDrivers === null);

  useEffect(() => {
    if (cachedDrivers) {
      setDrivers(cachedDrivers);
      setLoading(false);
      return;
    }
    if (!cachedPromise) {
      cachedPromise = ListDrivers()
        .then((list) => {
          cachedDrivers = list;
          setDrivers(list);
          setLoading(false);
        })
        .catch(() => {
          // 拉取失败：保持空列表（协议下拉仅 ssh/winrm），loading 置 false 避免卡死
          setLoading(false);
        });
    } else {
      void cachedPromise.then(() => {
        if (cachedDrivers) {
          setDrivers(cachedDrivers);
          setLoading(false);
        }
      });
    }
  }, []);

  const protocolSet = useMemo(
    () => new Set((drivers ?? []).map((d) => d.protocol.toLowerCase())),
    [drivers],
  );

  const isDB = useCallback(
    (protocol?: string) =>
      (protocol ? protocolSet.has(protocol.toLowerCase()) : false),
    [protocolSet],
  );

  return { drivers: drivers ?? [], loading, isDB };
}
