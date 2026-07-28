import { Typography } from 'antd';
const { Title, Paragraph } = Typography;

export default function Settings() {
  return (
    <div>
      <Title level={3}>设置</Title>
      <Paragraph>应用配置项</Paragraph>
    </div>
  );
}
