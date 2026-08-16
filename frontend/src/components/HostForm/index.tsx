import { Modal, Form, Input, InputNumber, Select, message, Row, Col } from 'antd';
import { useState } from 'react';
import type { CSSProperties } from 'react';
import { useTranslation } from 'react-i18next';
import type { hoststore, connector } from '@wailsjs/go/models';
import { useConnectors } from '@/hooks/useConnectors';

type HostInput = hoststore.HostInput;

interface HostFormProps {
  open: boolean;
  initialValues?: HostInput | null;
  onCancel: () => void;
  onSubmit: (input: HostInput) => Promise<void>;
  onTest: (input: HostInput) => Promise<boolean>;
}

const defaultValues: HostInput = {
  name: '',
  parentId: '',
  addr: '',
  port: 22,
  user: 'root',
  authType: 'password',
  secret: '',
  autoApprove: 'inherit',
  protocol: 'ssh',
  rdpPort: 3389,
  params: {},
};

// 按参数 schema 类型渲染控件。
// 必须接收并转发 Form.Item 注入的 value/onChange：Form.Item 通过 cloneElement
// 给直接子元素注入这两个 props，若自定义组件丢弃它们，用户输入不会写回表单
// store，触发 required 校验失败（现象：填了值仍提示非空）。
function ParamControl({
  p,
  value,
  onChange,
}: {
  p: connector.ParamSchema;
  value?: unknown;
  onChange?: (...args: any[]) => void;
}): React.JSX.Element {
  // 当前驱动仅使用 string/file 两种参数类型（mysql/postgres 的 database、sqlite 的 filePath），
  // int/secret/select 变体无驱动触发，已移除对应分支。
  return (
    <Input placeholder={p.placeholder} value={value as string} onChange={onChange} />
  );
}

export default function HostForm({
  open,
  initialValues,
  onCancel,
  onSubmit,
  onTest,
}: HostFormProps): React.JSX.Element {
  const [form] = Form.useForm<HostInput>();
  const [submitting, setSubmitting] = useState(false);
  const [testing, setTesting] = useState(false);
  const authType = Form.useWatch('authType', form);
  const protocol = Form.useWatch('protocol', form);
  const { t } = useTranslation('hosts');
  const { drivers } = useConnectors();
  const itemStyle: CSSProperties = { marginBottom: 12 };
  const secretIsKey = protocol === 'ssh' && authType === 'privatekey';

  // 当前协议的注册表驱动（无则 ssh/winrm 或未知）
  const selectedDriver = drivers.find((d) => d.protocol === protocol);
  const needsHost = selectedDriver?.needsHost ?? true;
  // 协议下拉：静态 ssh/winrm + 注册表驱动
  const protocolOptions = [
    { label: t('form.protocolSsh'), value: 'ssh' },
    { label: t('form.protocolWinrm'), value: 'winrm' },
    ...drivers.map((d) => ({ label: d.name, value: d.protocol })),
  ];

  const handleSubmit = async (): Promise<void> => {
    const values = await form.validateFields();
    if (values.protocol !== 'ssh') {
      values.authType = 'password';
    }
    setSubmitting(true);
    try {
      await onSubmit(values);
      message.success(t('form.saveSuccess'));
      onCancel();
    } catch (e) {
      message.error(t('form.saveFailed', { err: String(e) }));
    } finally {
      setSubmitting(false);
    }
  };

  const handleTest = async (): Promise<void> => {
    const values = await form.validateFields();
    if (initialValues && !values.secret && needsHost) {
      message.warning(t('form.testNeedSecret'));
      return;
    }
    setTesting(true);
    try {
      const ok = await onTest(values);
      if (ok) {
        message.success(t('form.testSuccess'));
      } else {
        message.warning(t('form.testFailed'));
      }
    } catch (e) {
      message.error(t('form.testError', { err: String(e) }));
    } finally {
      setTesting(false);
    }
  };

  // 协议切换：先清空上一协议的专属参数与凭据字段（secret/authType），避免陈旧值（如 SSH 私钥）随提交泄漏；ssh/winrm/DB 设置或清空端口
  const onProtocolChange = (value: string): void => {
    form.resetFields(['params', 'secret', 'authType']);
    if (value !== 'ssh') {
      form.setFieldsValue({ authType: 'password' });
    }
    if (value === 'winrm') {
      form.setFieldsValue({
        port: 5985,
        rdpPort: form.getFieldValue('rdpPort') || 3389,
      });
    } else if (value === 'ssh') {
      form.setFieldsValue({ port: 22 });
    } else {
      // 数据库驱动默认端口（sqlite 无端口，NeedsHost=false 隐藏）
      const portMap: Record<string, number> = { mysql: 3306, postgres: 5432 };
      form.setFieldsValue({ port: portMap[value] ?? undefined });
    }
  };

  return (
    <Modal
      title={initialValues ? t('form.titleEdit') : t('form.titleAdd')}
      width={480}
      open={open}
      onCancel={onCancel}
      onOk={handleSubmit}
      confirmLoading={submitting}
      destroyOnHidden
      afterOpenChange={(open) => {
        if (open) {
          form.resetFields();
          form.setFieldsValue(initialValues ?? defaultValues);
        }
      }}
    >
      <Form form={form} layout="vertical" size="small">
        <Row gutter={12}>
          <Col span={12}>
            <Form.Item
              name="name"
              label={t('form.name')}
              rules={[{ required: true, message: t('form.nameRequired') }]}
              style={itemStyle}
            >
              <Input placeholder="web-01 / app.db" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="protocol" label={t('form.protocol')} style={itemStyle}>
              <Select options={protocolOptions} onChange={onProtocolChange} />
            </Form.Item>
          </Col>
          {needsHost && (
            <>
              <Col span={12}>
                <Form.Item
                  name="addr"
                  label={t('form.addr')}
                  rules={[{ required: true, message: t('form.addrRequired') }]}
                  style={itemStyle}
                >
                  <Input placeholder="10.0.0.1 或 example.com" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name="port"
                  label={protocol === 'winrm' ? t('form.winrmPort') : t('form.port')}
                  rules={[{ required: true, message: t('form.portRequired') }]}
                  style={itemStyle}
                >
                  <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name="user"
                  label={t('form.user')}
                  rules={[{ required: true, message: t('form.userRequired') }]}
                  style={itemStyle}
                >
                  <Input placeholder="root" />
                </Form.Item>
              </Col>
              {protocol === 'ssh' && (
                <Col span={12}>
                  <Form.Item
                    name="authType"
                    label={t('form.authType')}
                    rules={[{ required: true }]}
                    style={itemStyle}
                  >
                    <Select
                      options={[
                        { label: t('form.password'), value: 'password' },
                        { label: t('form.privateKey'), value: 'privatekey' },
                      ]}
                    />
                  </Form.Item>
                </Col>
              )}
              <Col span={secretIsKey ? 24 : 12}>
                <Form.Item
                  name="secret"
                  label={
                    protocol !== 'ssh' || authType !== 'privatekey'
                      ? t('form.password')
                      : t('form.privateKey')
                  }
                  rules={[{ required: !initialValues, message: t('form.secretRequired') }]}
                  style={itemStyle}
                >
                  {protocol !== 'ssh' || authType !== 'privatekey' ? (
                    <Input.Password
                      placeholder={
                        initialValues
                          ? t('form.keepPasswordPlaceholder')
                          : t('form.passwordPlaceholder')
                      }
                    />
                  ) : (
                    <Input.TextArea
                      rows={3}
                      placeholder={
                        initialValues
                          ? t('form.keepSecretPlaceholder')
                          : '-----BEGIN RSA PRIVATE KEY-----...'
                      }
                    />
                  )}
                </Form.Item>
              </Col>
            </>
          )}
          {protocol === 'winrm' && (
            <Col span={12}>
              <Form.Item
                name="rdpPort"
                label={t('form.rdpPort')}
                rules={[{ required: true, message: t('form.rdpPortRequired') }]}
                style={itemStyle}
              >
                <InputNumber min={1} max={65535} placeholder="3389" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          )}
          {/* 注册表驱动的专属参数：按 schema 动态渲染到 params.<key> */}
          {selectedDriver?.params.map((p) => (
            <Col span={12} key={p.key}>
              <Form.Item
                name={['params', p.key]}
                label={p.label || p.key}
                rules={[{ required: !!p.required }]}
                initialValue={p.default}
                style={itemStyle}
              >
                <ParamControl p={p} />
              </Form.Item>
            </Col>
          ))}
          <Col span={24}>
            <Form.Item name="autoApprove" label={t('form.autoApproveLabel')} style={itemStyle}>
              <Select
                options={[
                  { label: t('form.autoApproveInherit'), value: 'inherit' },
                  { label: t('form.autoApproveOn'), value: 'on' },
                  { label: t('form.autoApproveOff'), value: 'off' },
                ]}
              />
            </Form.Item>
          </Col>
        </Row>
      </Form>
      <div style={{ textAlign: 'right', marginTop: 8 }}>
        <a onClick={handleTest} style={{ fontSize: 12 }}>
          {testing ? t('form.testing') : t('form.testConnection')}
        </a>
      </div>
    </Modal>
  );
}
