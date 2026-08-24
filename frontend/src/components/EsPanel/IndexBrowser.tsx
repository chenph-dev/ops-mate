import { Empty, List, Spin, Typography } from 'antd';
import { useTranslation } from 'react-i18next';

interface IndexBrowserProps {
  indices: string[];
  loading: boolean;
  selectedIndex: string;
  onSelectIndex: (index: string) => void;
}

/** 索引浏览器：显示集群索引列表，点选填充查询目标。 */
export default function IndexBrowser({
  indices,
  loading,
  selectedIndex,
  onSelectIndex,
}: IndexBrowserProps): React.JSX.Element {
  const { t } = useTranslation('es');

  return (
    <div
      style={{
        width: 220,
        flexShrink: 0,
        border: '1px solid var(--antd-color-border-secondary)',
        borderRadius: 4,
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <Typography.Text
        type="secondary"
        style={{ fontSize: 12, padding: '6px 10px', flexShrink: 0 }}
      >
        {t('browser.title')}
      </Typography.Text>
      <div
        style={{
          flex: 1,
          minHeight: 0,
          overflow: 'auto',
          borderTop: '1px solid var(--antd-color-border-secondary)',
        }}
      >
        {loading ? (
          <div style={{ display: 'flex', justifyContent: 'center', padding: 20 }}>
            <Spin size="small" />
          </div>
        ) : indices.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={<span style={{ fontSize: 12 }}>{t('browser.empty')}</span>}
            style={{ marginTop: 20 }}
          />
        ) : (
          <List
            size="small"
            dataSource={indices}
            renderItem={(index) => (
              <List.Item
                onClick={() => onSelectIndex(index)}
                style={{
                  cursor: 'pointer',
                  padding: '4px 10px',
                  fontSize: 12,
                  background:
                    index === selectedIndex
                      ? 'var(--antd-color-primary-bg)'
                      : undefined,
                  wordBreak: 'break-all',
                }}
              >
                {index}
              </List.Item>
            )}
          />
        )}
      </div>
    </div>
  );
}
