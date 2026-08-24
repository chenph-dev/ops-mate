import { useMemo, useState } from 'react';
import type { Key } from 'react';
import { Button, Spin, Tooltip, Tree, Typography } from 'antd';
import {
  CaretDownOutlined,
  CaretRightOutlined,
  EyeOutlined,
  TableOutlined,
} from '@ant-design/icons';
import type { TreeDataNode } from 'antd';
import { useTranslation } from 'react-i18next';
import type { dbexec } from '@wailsjs/go/models';

interface ObjectTreeProps {
  schema: dbexec.Schema | null;
  loading: boolean;
  error: string;
  onOpenTable: (name: string) => void;
}

/**
 * 左侧对象树：始终显示「表」「视图」两个根节点（空组展开显示空），默认全部收缩；
 * 顶部提供全部展开/全部收缩工具。双击表/视图打开数据浏览。
 */
export default function ObjectTree({
  schema,
  loading,
  error,
  onOpenTable,
}: ObjectTreeProps): React.JSX.Element {
  const { t } = useTranslation('hosts');
  const [expandedKeys, setExpandedKeys] = useState<Key[]>([]);

  const treeData = useMemo<TreeDataNode[]>(() => {
    const objects = schema?.tables ?? [];
    const build = (
      groupKey: string,
      title: string,
      list: typeof objects,
      icon: React.ReactNode,
    ): TreeDataNode => ({
      key: groupKey,
      title,
      icon,
      children: list.map((obj) => ({
        key: `${groupKey}:${obj.name}`,
        title: obj.name,
        icon,
        children: obj.columns.map((c) => ({
          key: `${groupKey}:${obj.name}:${c.name}`,
          title: `${c.name} : ${c.dataType}`,
          isLeaf: true,
        })),
      })),
    });
    const tables = objects.filter((o) => o.type !== 'view');
    const views = objects.filter((o) => o.type === 'view');
    return [
      build('table', t('db.tree.tables'), tables, <TableOutlined />),
      build('view', t('db.tree.views'), views, <EyeOutlined />),
    ];
  }, [schema, t]);

  // 全部展开的目标 keys：分组 + 各表/视图（空 children 节点也能展开显示空）
  const expandableKeys = useMemo<Key[]>(() => {
    const keys: Key[] = ['table', 'view'];
    (schema?.tables ?? []).forEach((o) => {
      keys.push(`${o.type === 'view' ? 'view' : 'table'}:${o.name}`);
    });
    return keys;
  }, [schema]);

  const handleDoubleClick = (_: unknown, node: TreeDataNode): void => {
    const key = String(node.key ?? '');
    if (key.startsWith('table:') || key.startsWith('view:')) {
      onOpenTable(key.slice(key.indexOf(':') + 1));
    }
  };

  return (
    <div
      style={{
        width: 200,
        flexShrink: 0,
        border: '1px solid var(--antd-color-border-secondary)',
        borderRadius: 4,
        overflow: 'auto',
        padding: 4,
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 2,
          padding: '0 2px',
          flexShrink: 0,
        }}
      >
        <Typography.Text
          type="secondary"
          style={{ fontSize: 12, flex: 1 }}
        >
          {t('db.schema')}
        </Typography.Text>
        <Tooltip title={t('db.tree.expandAll')}>
          <Button
            type="text"
            size="small"
            icon={<CaretRightOutlined />}
            onClick={() => setExpandedKeys(expandableKeys)}
          />
        </Tooltip>
        <Tooltip title={t('db.tree.collapseAll')}>
          <Button
            type="text"
            size="small"
            icon={<CaretDownOutlined />}
            onClick={() => setExpandedKeys([])}
          />
        </Tooltip>
      </div>
      {loading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 20 }}>
          <Spin size="small" />
        </div>
      ) : error ? (
        <div
          style={{
            color: 'var(--antd-color-error)',
            fontSize: 12,
            whiteSpace: 'pre-wrap',
            padding: 4,
          }}
        >
          {error}
        </div>
      ) : (
        <Tree
          treeData={treeData}
          expandedKeys={expandedKeys}
          onExpand={setExpandedKeys}
          onDoubleClick={handleDoubleClick}
          showLine
        />
      )}
    </div>
  );
}
