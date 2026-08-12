import { useEffect, useState } from 'react';
import {
  Form,
  Input,
  Select,
  Button,
  Card,
  message,
  Space,
  Switch,
  Typography,
} from 'antd';
import { useTranslation } from 'react-i18next';
import { GetAIConfig, SaveAIConfig } from '@wailsjs/go/handler/AIConfigHandler';
import {
  GetApprovalPolicy,
  SaveApprovalPolicy,
} from '@wailsjs/go/handler/ApprovalPolicyHandler';
import type { configstore } from '@wailsjs/go/models';

type AIConfig = configstore.AIConfig;
type ApprovalPolicy = configstore.ApprovalPolicy;

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

  const [policy, setPolicy] = useState<ApprovalPolicy>({
    enableAuto: true,
    readOnlyList: [],
  });
  const [policyDraft, setPolicyDraft] = useState('');
  const [savingPolicy, setSavingPolicy] = useState(false);

  useEffect(() => {
    GetAIConfig()
      .then((cfg) => {
        form.setFieldsValue(cfg);
      })
      .catch(() => {});
  }, [form]);

  useEffect(() => {
    GetApprovalPolicy()
      .then((p) => {
        setPolicy(p);
        setPolicyDraft(p.readOnlyList?.join(', ') ?? '');
      })
      .catch(() => {});
  }, []);

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

      <Card size="small" style={{ marginTop: 16 }}>
        <Title level={5} style={{ marginTop: 0 }}>审批策略</Title>
        <Paragraph type="secondary" style={{ fontSize: 12 }}>
          开启后，AI 提议的只读命令（命中下方白名单且不含管道/重定向）将自动执行并展示「已自动执行」卡片；写操作与高危命令仍逐一审批。
        </Paragraph>
        <Form layout="vertical" size="small">
          <Form.Item label="自动放行只读命令">
            <Switch
              checked={policy.enableAuto}
              onChange={(v) => setPolicy({ ...policy, enableAuto: v })}
            />
          </Form.Item>
          <Form.Item
            label="只读命令白名单"
            extra="逗号或空格分隔；留空使用内置默认（ls/df/free/tail/cat/ps 等）"
          >
            <Input.TextArea
              value={policyDraft}
              onChange={(e) => setPolicyDraft(e.target.value)}
              autoSize={{ minRows: 3, maxRows: 6 }}
              placeholder="ls, df, free, tail, cat, ps"
            />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              loading={savingPolicy}
              onClick={async () => {
                setSavingPolicy(true);
                try {
                  const list = policyDraft
                    .split(/[\s,]+/)
                    .map((s) => s.trim())
                    .filter(Boolean);
                  await SaveApprovalPolicy({
                    enableAuto: policy.enableAuto,
                    readOnlyList: list,
                  });
                  message.success('审批策略已保存');
                } catch (e) {
                  message.error(`保存失败: ${e}`);
                } finally {
                  setSavingPolicy(false);
                }
              }}
            >
              保存审批策略
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
