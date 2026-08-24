import { Button, Empty, Input, List, Spin, Tooltip } from 'antd';
import { RightOutlined, SearchOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

interface KeyBrowserProps {
  keys: string[];
  loading: boolean;
  /** 当前 SCAN 游标；"0" 表示扫描结束（无下一页）。 */
  cursor: string;
  pattern: string;
  selectedKey: string | null;
  onPatternChange: (p: string) => void;
  onSearch: () => void;
  onNext: () => void;
  onSelectKey: (key: string) => void;
}

/** 键空间浏览器：pattern 过滤 + SCAN 游标分页浏览 keys。 */
export default function KeyBrowser({
  keys,
  loading,
  cursor,
  pattern,
  selectedKey,
  onPatternChange,
  onSearch,
  onNext,
  onSelectKey,
}: KeyBrowserProps): React.JSX.Element {
  const { t } = useTranslation('redis');

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
      <div style={{ padding: 6, display: 'flex', gap: 4 }}>
        <Input
          size="small"
          placeholder={t('browser.patternPlaceholder')}
          value={pattern}
          onChange={(e) => onPatternChange(e.target.value)}
          onPressEnter={onSearch}
          allowClear
        />
        <Tooltip title={t('browser.search')}>
          <Button size="small" icon={<SearchOutlined />} onClick={onSearch} />
        </Tooltip>
      </div>
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
        ) : keys.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={<span style={{ fontSize: 12 }}>{t('browser.empty')}</span>}
            style={{ marginTop: 20 }}
          />
        ) : (
          <List
            size="small"
            dataSource={keys}
            renderItem={(key) => (
              <List.Item
                onClick={() => onSelectKey(key)}
                style={{
                  cursor: 'pointer',
                  padding: '4px 10px',
                  fontSize: 12,
                  background:
                    key === selectedKey
                      ? 'var(--antd-color-primary-bg)'
                      : undefined,
                  wordBreak: 'break-all',
                }}
              >
                {key}
              </List.Item>
            )}
          />
        )}
      </div>
      <div
        style={{
          padding: 6,
          borderTop: '1px solid var(--antd-color-border-secondary)',
        }}
      >
        <Button
          size="small"
          block
          disabled={cursor === '0' || loading}
          onClick={onNext}
          icon={<RightOutlined />}
        >
          {t('browser.next')}
        </Button>
      </div>
    </div>
  );
}
