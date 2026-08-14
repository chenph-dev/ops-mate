import { Button, Tooltip, theme } from 'antd';
import { useTranslation } from 'react-i18next';
import {
  ClearOutlined,
  CopyOutlined,
  DisconnectOutlined,
  FolderOpenOutlined,
  ReloadOutlined,
  RobotOutlined,
  SearchOutlined,
  ZoomInOutlined,
  ZoomOutOutlined,
} from '@ant-design/icons';

interface TerminalHeaderProps {
  hostName: string;
  hostAddr: string;
  statusDot: string;
  statusText: string;
  connected: boolean;
  fontSize: number;
  maxFontSize: number;
  minFontSize: number;
  aiOpen: boolean;
  onToggleAI: () => void;
  onRefresh: () => void;
  onOpenSftp: () => void;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onSearch: () => void;
  onClear: () => void;
  onCopy: () => Promise<void>;
  onDisconnect: () => void;
}

/** 终端标题栏：资产信息 + 智能体面板开关/刷新/缩放/搜索/清空/复制/断开操作。 */
export default function TerminalHeader({
  hostName,
  hostAddr,
  statusDot,
  statusText,
  connected,
  fontSize,
  maxFontSize,
  minFontSize,
  aiOpen,
  onToggleAI,
  onRefresh,
  onOpenSftp,
  onZoomIn,
  onZoomOut,
  onSearch,
  onClear,
  onCopy,
  onDisconnect,
}: TerminalHeaderProps): React.JSX.Element {
  const { token } = theme.useToken();
  const { t } = useTranslation('terminal');
  return (
    <div
      style={{
        padding: '4px 10px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        borderBottom: `1px solid ${token.colorBorderSecondary}`,
        flexShrink: 0,
        background: token.colorBgElevated,
      }}
    >
      <div
        style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}
      >
        <span
          style={{
            fontSize: 11,
            color: token.colorTextSecondary,
            whiteSpace: 'nowrap',
          }}
        >
          {t('header.title')}
        </span>
        <span style={{ fontSize: 12, fontWeight: 600, whiteSpace: 'nowrap' }}>
          {hostName || t('header.noHost')}
        </span>
        <span
          style={{
            fontSize: 11,
            color: connected ? token.colorSuccess : token.colorTextSecondary,
            whiteSpace: 'nowrap',
          }}
        >
          {statusDot} {statusText}
        </span>
        {hostAddr && (
          <Tooltip title={hostAddr}>
            <span
              style={{
                fontSize: 11,
                color: token.colorTextTertiary,
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              {hostAddr}
            </span>
          </Tooltip>
        )}
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 2 }}>
        <Tooltip title={t('header.sftp')}>
          <Button
            type="text"
            size="small"
            icon={<FolderOpenOutlined />}
            onClick={onOpenSftp}
          />
        </Tooltip>
        <Tooltip title={aiOpen ? t('header.aiOpen') : t('header.aiClose')}>
          <Button
            type={aiOpen ? 'primary' : 'text'}
            size="small"
            icon={<RobotOutlined />}
            onClick={onToggleAI}
          />
        </Tooltip>
        <Tooltip title={t('header.refresh')}>
          <Button
            type="text"
            size="small"
            icon={<ReloadOutlined />}
            onClick={onRefresh}
          />
        </Tooltip>
        <Tooltip title={t('header.zoomIn')}>
          <Button
            type="text"
            size="small"
            icon={<ZoomInOutlined />}
            onClick={onZoomIn}
            disabled={fontSize >= maxFontSize}
          />
        </Tooltip>
        <Tooltip title={t('header.zoomOut')}>
          <Button
            type="text"
            size="small"
            icon={<ZoomOutOutlined />}
            onClick={onZoomOut}
            disabled={fontSize <= minFontSize}
          />
        </Tooltip>
        <Tooltip title={t('header.search')}>
          <Button
            type="text"
            size="small"
            icon={<SearchOutlined />}
            onClick={onSearch}
          />
        </Tooltip>
        <Tooltip title={t('header.clear')}>
          <Button
            type="text"
            size="small"
            icon={<ClearOutlined />}
            onClick={onClear}
          />
        </Tooltip>
        <Tooltip title={t('header.copy')}>
          <Button
            type="text"
            size="small"
            icon={<CopyOutlined />}
            onClick={() => void onCopy()}
          />
        </Tooltip>
        {connected && (
          <Tooltip title={t('header.disconnect')}>
            <Button
              type="text"
              size="small"
              danger
              icon={<DisconnectOutlined />}
              onClick={onDisconnect}
            />
          </Tooltip>
        )}
      </div>
    </div>
  );
}
