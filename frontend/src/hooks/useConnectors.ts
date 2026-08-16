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
  isDB: (protocol?: string) => boolean;
} {
  const [drivers, setDrivers] = useState<DriverMeta[]>(cachedDrivers ?? []);

  useEffect(() => {
    if (cachedDrivers) {
      setDrivers(cachedDrivers);
      return;
    }
    if (!cachedPromise) {
      cachedPromise = ListDrivers()
        .then((list) => {
          cachedDrivers = list;
          setDrivers(list);
        })
        .catch(() => {
          // 拉取失败：重置缓存允许下次挂载重试（否则本次会话内
          // 协议注册表永久为空、新挂载组件拿不到注册表）
          cachedPromise = null;
        });
    } else {
      void cachedPromise.then(() => {
        if (cachedDrivers) {
          setDrivers(cachedDrivers);
        }
      });
    }
  }, []);

  // kindOf 返回协议的驱动类型（"db"/"command"）；未注册/未知/空 → undefined。
  const kindOf = useCallback(
    (protocol?: string): string | undefined => {
      if (!protocol) return undefined;
      const p = protocol.toLowerCase();
      return (drivers ?? []).find((d) => d.protocol.toLowerCase() === p)?.kind;
    },
    [drivers],
  );

  // isDB 只认后端归一化的 kind==='db'：未知协议 → undefined → false（保守安全）。
  const isDB = useCallback(
    (protocol?: string) => kindOf(protocol) === 'db',
    [kindOf],
  );

  return { drivers: drivers ?? [], isDB };
}
