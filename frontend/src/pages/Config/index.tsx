import { useEffect, useState } from 'react';
import { Form, Input, Select, Button, Card, message, Space, Typography } from 'antd';
import { GetAIConfig, SaveAIConfig } from '@wailsjs/go/handler/AIConfigHandler';
import type { configstore } from '@wailsjs/go/models';

type AIConfig = configstore.AIConfig;

const { Title, Paragraph } = Typography;

const providers = [
  { label: 'Ollama (本地)', value: 'ollama' },
  { label: 'Claude (Anthropic)', value: 'claude' },
  { label: 'OpenAI', value: 'openai' },
  { label: 'DeepSeek', value: 'deepseek' },
  { label: '通义千问 (DashScope)', value: 'dashscope' },
  { label: '智谱 AI', value: 'zhipu' },
];

export default function ConfigPage(): React.JSX.Element {
  const [form] = Form.useForm<AIConfig>();
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    GetAIConfig().then((cfg) => {
      form.setFieldsValue(cfg);
    }).catch(() => {});
  }, [form]);

  const handleSave = async (): Promise<void> => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await SaveAIConfig(values);
      message.success('配置已保存');
    } catch (e) {
      message.error(`保存失败: ${e}`);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div style={{ maxWidth: 560, margin: '0 auto' }}>
      <Title level={4}>AI 配置</Title>
      <Paragraph type="secondary" style={{ fontSize: 12 }}>
        配置 AI 提供商。更改后立即生效。
      </Paragraph>

      <Card size="small">
        <Form form={form} layout="vertical" size="small">
          <Form.Item name="provider" label="提供商" rules={[{ required: true, message: '请选择提供商' }]}>
            <Select options={providers} placeholder="选择 AI 提供商" />
          </Form.Item>

          <Form.Item name="model" label="模型" rules={[{ required: true, message: '请输入模型名称' }]}>
            <Input placeholder="claude-sonnet-5 / llama3 / gpt-4 ..." />
          </Form.Item>

          <Form.Item name="baseURL" label="Base URL">
            <Input placeholder="留空使用默认地址" />
          </Form.Item>

          <Form.Item name="apiKey" label="API Key">
            <Input.Password placeholder="输入 API Key（本地 Ollama 可留空）" />
          </Form.Item>

          <Form.Item>
            <Space>
              <Button type="primary" onClick={handleSave} loading={saving}>
                保存配置
              </Button>
              <Button onClick={() => GetAIConfig().then((cfg) => form.setFieldsValue(cfg))}>
                重置
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
