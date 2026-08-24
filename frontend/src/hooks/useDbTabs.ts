import { useCallback, useEffect, useRef, useState } from 'react';

/** DB 工作台标签类型：query=查询编辑器，table=表/视图数据浏览。 */
export type DbTabType = 'query' | 'table';

export interface DbTab {
  key: string;
  type: DbTabType;
  title: string;
  /** table 类型：表/视图名 */
  tableName?: string;
}

interface UseDbTabsReturn {
  tabs: DbTab[];
  activeKey: string | null;
  newQuery: () => void;
  openTable: (name: string) => void;
  closeTab: (key: string) => void;
  activate: (key: string) => void;
}

let dbTabSeq = 0;
function nextKey(): string {
  dbTabSeq += 1;
  return `db-${dbTabSeq}`;
}

/** 初始查询标签 key（固定值，不与 nextKey 生成的 db-N 冲突）。 */
const INITIAL_KEY = 'db-initial';

/**
 * DB 工作台多标签管理（查询 / 表数据）。
 * 仿 useTerminalSessions 的 tabs/activeKey/refs 模式；各标签的运行时状态
 * （SQL、结果、分页）由对应子组件内部维护（antd Tabs 默认不销毁非活动 pane）。
 */
export function useDbTabs(): UseDbTabsReturn {
  // 初始即带一个查询标签（避免在 DbPanel 用 effect 初始化——StrictMode 下 effect
  // 双跑会各自捕获空 tabs 各加一个，导致默认开两个页签）。
  const [tabs, setTabs] = useState<DbTab[]>([
    { key: INITIAL_KEY, type: 'query', title: '查询' },
  ]);
  const [activeKey, setActiveKey] = useState<string | null>(INITIAL_KEY);
  const tabsRef = useRef<DbTab[]>([]);
  useEffect(() => {
    tabsRef.current = tabs;
  }, [tabs]);

  /** 新建一个查询标签并激活。 */
  const newQuery = useCallback((): void => {
    const key = nextKey();
    setTabs((prev) => [...prev, { key, type: 'query', title: '查询' }]);
    setActiveKey(key);
  }, []);

  /** 打开（或激活已打开）某表/视图的数据浏览标签。 */
  const openTable = useCallback((name: string): void => {
    const existing = tabsRef.current.find(
      (t) => t.type === 'table' && t.tableName === name,
    );
    if (existing) {
      setActiveKey(existing.key);
      return;
    }
    const key = nextKey();
    setTabs((prev) => [
      ...prev,
      { key, type: 'table', title: name, tableName: name },
    ]);
    setActiveKey(key);
  }, []);

  /** 关闭标签；关闭的是激活标签时激活相邻标签。 */
  const closeTab = useCallback((key: string): void => {
    setTabs((prev) => {
      const idx = prev.findIndex((t) => t.key === key);
      const next = prev.filter((t) => t.key !== key);
      setActiveKey((cur) => {
        if (cur !== key) return cur;
        return next[Math.min(idx, next.length - 1)]?.key ?? null;
      });
      return next;
    });
  }, []);

  const activate = useCallback((key: string): void => setActiveKey(key), []);

  return { tabs, activeKey, newQuery, openTable, closeTab, activate };
}
