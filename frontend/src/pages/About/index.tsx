import { Typography, Card, Divider, Space } from 'antd';
import { useTranslation } from 'react-i18next';
import { GithubOutlined } from '@ant-design/icons';

const { Title, Paragraph, Text, Link } = Typography;

/** 功能特性列表（title/desc 为 about 命名空间下的 i18n key）。 */
const FEATURES: Array<{ icon: string; title: string; desc: string }> = [
  { icon: '🖥️', title: 'feat.hostsTitle', desc: 'feat.hostsDesc' },
  { icon: '💻', title: 'feat.terminalTitle', desc: 'feat.terminalDesc' },
  { icon: '📁', title: 'feat.sftpTitle', desc: 'feat.sftpDesc' },
  { icon: '🤖', title: 'feat.aiTitle', desc: 'feat.aiDesc' },
];

export default function About(): React.JSX.Element {
  const { t } = useTranslation('about');
  return (
    <div style={{ maxWidth: 560, margin: '0 auto' }}>
      <Title level={4}>{t('title')}</Title>
      <Card size="small">
        <Space direction="vertical" size={4} style={{ width: '100%' }}>
          <Paragraph style={{ margin: 0 }}>
            <Text strong>ops-mate</Text>
          </Paragraph>
          <Paragraph type="secondary" style={{ fontSize: 12, margin: 0 }}>
            {t('tagline')}
          </Paragraph>

          <Divider style={{ margin: '8px 0' }} />

          {FEATURES.map((f) => (
            <Paragraph key={f.title} style={{ margin: 0, fontSize: 12 }}>
              <Text strong>
                {f.icon} {t(f.title)}
              </Text>
              <Text type="secondary"> — {t(f.desc)}</Text>
            </Paragraph>
          ))}

          <Divider style={{ margin: '8px 0' }} />

          <Paragraph type="secondary" style={{ fontSize: 12, margin: 0 }}>
            {t('techstack')}
          </Paragraph>
          <Link href="https://github.com" target="_blank">
            <GithubOutlined /> GitHub
          </Link>
        </Space>
      </Card>
    </div>
  );
}
