import { Typography, Card, Divider, Space } from 'antd';
import { GithubOutlined } from '@ant-design/icons';

const { Title, Paragraph, Text, Link } = Typography;

/** 功能特性列表 */
const FEATURES: Array<{ icon: string; title: string; desc: string }> = [
  {
    icon: '🖥️',
    title: '主机管理',
    desc: '文件夹树状组织主机、连接测试、命令执行',
  },
  {
    icon: '💻',
    title: 'SSH 终端',
    desc: '交互式终端，动态缩放、搜索、命令历史',
  },
  {
    icon: '📁',
    title: 'SFTP 传输',
    desc: '远程文件浏览与上传下载，并发队列 + 进度控制',
  },
  {
    icon: '🤖',
    title: 'AI 运维智能体',
    desc: '命令审批 / 计划模式，每一步都经你确认',
  },
];

export default function About(): React.JSX.Element {
  return (
    <div style={{ maxWidth: 560, margin: '0 auto' }}>
      <Title level={4}>关于 ops-mate</Title>
      <Card size="small">
        <Space direction="vertical" size={4} style={{ width: '100%' }}>
          <Paragraph style={{ margin: 0 }}>
            <Text strong>ops-mate</Text>
          </Paragraph>
          <Paragraph type="secondary" style={{ fontSize: 12, margin: 0 }}>
            基于 Wails + React 的 AI 运维智能体，通过 AI 对话远程管理 Linux
            主机。
          </Paragraph>

          <Divider style={{ margin: '8px 0' }} />

          {FEATURES.map((f) => (
            <Paragraph key={f.title} style={{ margin: 0, fontSize: 12 }}>
              <Text strong>
                {f.icon} {f.title}
              </Text>
              <Text type="secondary"> — {f.desc}</Text>
            </Paragraph>
          ))}

          <Divider style={{ margin: '8px 0' }} />

          <Paragraph type="secondary" style={{ fontSize: 12, margin: 0 }}>
            技术栈: Go + Eino | React + Ant Design + xterm.js | SQLite + GORM
          </Paragraph>
          <Link href="https://github.com" target="_blank">
            <GithubOutlined /> GitHub
          </Link>
        </Space>
      </Card>
    </div>
  );
}
