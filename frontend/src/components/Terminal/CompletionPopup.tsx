import { useEffect, useRef } from 'react';
import { theme } from 'antd';
import { useTranslation } from 'react-i18next';

interface CommandMatch {
  name: string;
  desc: string;
  text: string;
  type: 'command' | 'template';
}

interface CompletionPopupProps {
  x: number;
  y: number;
  matches: CommandMatch[];
  selectedIndex: number;
  prefix: string;
  onSelect: (match: CommandMatch) => void;
}

export default function CompletionPopup({
  x,
  y,
  matches,
  selectedIndex,
  prefix,
  onSelect,
}: CompletionPopupProps): React.JSX.Element {
  const { token } = theme.useToken();
  const { t } = useTranslation('terminal');
  const selectedRef = useRef<HTMLDivElement | null>(null);

  // 选中项变化时滚动到可视区，避免方向键选择超出弹窗可视范围
  useEffect(() => {
    selectedRef.current?.scrollIntoView({ block: 'nearest' });
  }, [selectedIndex]);

  return (
    <div
      style={{
        position: 'fixed',
        left: x,
        top: y,
        zIndex: 9999,
        background: token.colorBgElevated,
        border: `1px solid ${token.colorBorderSecondary}`,
        borderRadius: token.borderRadiusLG,
        boxShadow: token.boxShadowSecondary,
        minWidth: 240,
        maxHeight: 240,
        overflow: 'auto',
        padding: '4px 0',
        fontSize: 13,
      }}
      onMouseDown={(e): void => e.preventDefault()}
    >
      {matches.map((match, index) => {
        const selected = index === selectedIndex;
        const highlightLen = Math.min(prefix.length, match.name.length);
        const prefixPart = match.name.slice(0, highlightLen);
        const restPart = match.name.slice(highlightLen);
        return (
          <div
            key={`${match.type}-${match.name}-${index}`}
            ref={selected ? selectedRef : undefined}
            onClick={(): void => onSelect(match)}
            style={{
              padding: '5px 12px',
              cursor: 'pointer',
              background: selected ? token.colorPrimaryBg : undefined,
              display: 'flex',
              alignItems: 'baseline',
              gap: 10,
            }}
          >
            <span
              style={{
                fontFamily:
                  '"Cascadia Code", "Fira Code", "Consolas", "Monaco", monospace',
                whiteSpace: 'nowrap',
                color: token.colorText,
              }}
            >
              <span style={{ color: token.colorPrimary }}>{prefixPart}</span>
              <span>{restPart}</span>
            </span>
            {match.desc && (
              <span
                style={{
                  color: token.colorTextSecondary,
                  fontSize: 12,
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  flex: 1,
                  minWidth: 0,
                }}
              >
                {match.desc}
              </span>
            )}
          </div>
        );
      })}
      {matches.length > 0 && (
        <div
          style={{
            padding: '3px 12px',
            fontSize: 11,
            color: token.colorTextTertiary,
            borderTop: `1px solid ${token.colorBorderSecondary}`,
          }}
        >
          {t('completion.hint', { count: matches.length })}
        </div>
      )}
    </div>
  );
}
