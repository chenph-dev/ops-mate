import { Typography } from 'antd';
const { Title, Paragraph } = Typography;

export default function Home() {
  return (
    <div>
      <Title level={3}>首页</Title>
      <Paragraph>欢迎使用 ops-mate</Paragraph>
    </div>
  );
}
