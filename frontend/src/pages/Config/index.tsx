import { useEffect, useState } from 'react';
import { Form, Input, Select, Button, Card, message, Space, Typography } from 'antd';
import { GetAIConfig, SaveAIConfig } from '@wailsjs/go/handler/AIConfigHandler';
import type { configstore } from '@wailsjs/go/models';

type AIConfig = configstore.AIConfig;

const { Title, Paragraph } = Typography;

// LLM 模型供应商（provider 值直接对应后端 provider.go 的 NewChatModel 分支）。
// 具体服务商（OpenAI/DeepSeek/通义/智谱/Ollama 等）通过 BaseURL + Model 区分。
const protocols = [
  { label: 'OpenAI 兼容', value: 'openai' },
  { label: 'Anthropic', value: 'claude' },
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
      <Title level={4}>LLM模型配置</Title>
      <Paragraph type="secondary" style={{ fontSize: 12 }}>
        配置 AI 提供商。更改后立即生效。
      </Paragraph>

      <Card size="small">
        <Form form={form} layout="vertical" size="small">
          <Form.Item name="provider" label="LLM模型供应商" rules={[{ required: true, message: '请选择 LLM 模型供应商' }]}>
            <Select options={protocols} placeholder="选择 LLM 模型供应商" />
          </Form.Item>

          <Form.Item name="baseURL" label="Base URL">
            <Input placeholder="完整地址（含 /v1），如 https://api.deepseek.com/v1；留空用默认" />
          </Form.Item>

          <Form.Item name="apiKey" label="API Key">
            <Input.Password placeholder="输入 API Key（本地服务可留空）" />
          </Form.Item>

          <Form.Item name="model" label="模型" rules={[{ required: true, message: '请输入模型名称' }]}>
            <Input placeholder="如 claude-sonnet-5 / gpt-4o / deepseek-chat" />
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
