import { useEffect, useState } from 'react';
import {
  Form,
  Input,
  Select,
  Button,
  Card,
  message,
  Space,
  Typography,
} from 'antd';
import { useTranslation } from 'react-i18next';
import { GetAIConfig, SaveAIConfig } from '@wailsjs/go/handler/AIConfigHandler';
import type { configstore } from '@wailsjs/go/models';

type AIConfig = configstore.AIConfig;

const { Title, Paragraph } = Typography;

// LLM 模型供应商（provider 值直接对应后端 provider.go 的 NewChatModel 分支）。
// 具体服务商（OpenAI/DeepSeek/通义/智谱/Ollama 等）通过 BaseURL + Model 区分。
// label 存 i18n key（config 命名空间），渲染处用 t(label) 取当前语言文案。
const protocolKeys = [
  { label: 'providerOpenAI', value: 'openai' },
  { label: 'providerClaude', value: 'claude' },
];

export default function ConfigPage(): React.JSX.Element {
  const [form] = Form.useForm<AIConfig>();
  const [saving, setSaving] = useState(false);
  const { t } = useTranslation('config');

  const protocols = protocolKeys.map((p) => ({ ...p, label: t(p.label) }));

  useEffect(() => {
    GetAIConfig()
      .then((cfg) => {
        form.setFieldsValue(cfg);
      })
      .catch(() => {});
  }, [form]);

  const handleSave = async (): Promise<void> => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await SaveAIConfig(values);
      message.success(t('saveSuccess'));
    } catch (e) {
      message.error(t('saveFailed', { err: String(e) }));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div style={{ maxWidth: 560, margin: '0 auto' }}>
      <Title level={4}>{t('title')}</Title>
      <Paragraph type="secondary" style={{ fontSize: 12 }}>
        {t('subtitle')}
      </Paragraph>

      <Card size="small">
        <Form form={form} layout="vertical" size="small">
          <Form.Item
            name="provider"
            label={t('provider')}
            rules={[{ required: true, message: t('providerRequired') }]}
          >
            <Select options={protocols} placeholder={t('providerPlaceholder')} />
          </Form.Item>

          <Form.Item name="baseURL" label="Base URL">
            <Input placeholder={t('baseURLPlaceholder')} />
          </Form.Item>

          <Form.Item name="apiKey" label="API Key">
            <Input.Password placeholder={t('apiKeyPlaceholder')} />
          </Form.Item>

          <Form.Item
            name="model"
            label={t('model')}
            rules={[{ required: true, message: t('modelRequired') }]}
          >
            <Input placeholder={t('modelPlaceholder')} />
          </Form.Item>

          <Form.Item>
            <Space>
              <Button type="primary" onClick={handleSave} loading={saving}>
                {t('save')}
              </Button>
              <Button
                onClick={() =>
                  GetAIConfig().then((cfg) => form.setFieldsValue(cfg))
                }
              >
                {t('reset')}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
