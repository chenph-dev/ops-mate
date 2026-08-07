import { Button, Input, theme, type InputRef } from 'antd';
import { UpOutlined, DownOutlined, CloseOutlined } from '@ant-design/icons';

interface SearchBarProps {
  searchText: string;
  searchInputRef: React.RefObject<InputRef | null>;
  onSearchChange: (value: string) => void;
  onSearchNext: () => void;
  onSearchPrev: () => void;
  onSearchClose: () => void;
}

/** 终端内容搜索栏。 */
export default function SearchBar({
  searchText,
  searchInputRef,
  onSearchChange,
  onSearchNext,
  onSearchPrev,
  onSearchClose,
}: SearchBarProps): React.JSX.Element {
  const { token } = theme.useToken();
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        padding: '4px 10px',
        borderBottom: `1px solid ${token.colorBorderSecondary}`,
        background: token.colorBgElevated,
        flexShrink: 0,
      }}
    >
      <Input
        ref={searchInputRef}
        size="small"
        placeholder="搜索终端内容..."
        value={searchText}
        onChange={(e): void => onSearchChange(e.target.value)}
        onKeyDown={(e): void => {
          if (e.key === 'Enter') {
            e.preventDefault();
            onSearchNext();
          }
        }}
        style={{ width: 200 }}
        allowClear
      />
      <Button
        type="text"
        size="small"
        icon={<DownOutlined />}
        onClick={onSearchNext}
      />
      <Button
        type="text"
        size="small"
        icon={<UpOutlined />}
        onClick={onSearchPrev}
      />
      <Button
        type="text"
        size="small"
        icon={<CloseOutlined />}
        onClick={onSearchClose}
      />
    </div>
  );
}
