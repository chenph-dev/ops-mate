import { useTranslation } from 'react-i18next';

/** 404 页：独立文件以便语言切换后文案随 useTranslation 刷新。 */
export default function NotFound(): React.JSX.Element {
  const { t } = useTranslation('common');
  return <div style={{ padding: 24 }}>{t('pageNotFound')}</div>;
}
