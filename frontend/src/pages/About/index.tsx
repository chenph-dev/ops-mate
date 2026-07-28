import { Typography } from 'antd';
const { Title, Paragraph } = Typography;

export default function About() {
  return (
    <div>
      <Title level={3}>关于</Title>
      <Paragraph>ops-mate — 基于 Wails + React 的桌面应用</Paragraph>
    </div>
  );
}
