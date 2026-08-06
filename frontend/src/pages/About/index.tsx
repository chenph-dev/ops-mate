import { Typography, Card, Space } from 'antd';
import { GithubOutlined } from '@ant-design/icons';

const { Title, Paragraph, Text, Link } = Typography;

export default function About(): React.JSX.Element {
  return (
    <div style={{ maxWidth: 560, margin: '0 auto' }}>
      <Title level={4}>关于 ops-mate</Title>
      <Card size="small">
        <Space direction="vertical" size={4}>
          <Paragraph style={{ margin: 0 }}>
            <Text strong>ops-mate</Text> — 基于 Wails + React 的 AI 运维智能体
          </Paragraph>
          <Paragraph type="secondary" style={{ fontSize: 12, margin: 0 }}>
            通过 AI 对话远程管理 Linux 主机，支持命令审批、历史记忆、多主机管理。
          </Paragraph>
          <Paragraph type="secondary" style={{ fontSize: 12, margin: 0 }}>
            技术栈: Go + Eino | React + Ant Design | SQLite + GORM
          </Paragraph>
          <Link href="https://github.com" target="_blank">
            <GithubOutlined /> GitHub
          </Link>
        </Space>
      </Card>
    </div>
  );
}
