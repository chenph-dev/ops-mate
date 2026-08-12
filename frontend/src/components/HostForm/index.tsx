import { Modal, Form, Input, InputNumber, Select, message } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { hoststore } from '@wailsjs/go/models';

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
};

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
  const { t } = useTranslation('hosts');

  const handleSubmit = async (): Promise<void> => {
    const values = await form.validateFields();
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
    // 编辑模式：secret 留空表示不修改，测试连接需要真实凭据，提示先输入
    if (initialValues && !values.secret) {
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

  return (
    <Modal
      title={initialValues ? t('form.titleEdit') : t('form.titleAdd')}
      open={open}
      onCancel={onCancel}
      onOk={handleSubmit}
      confirmLoading={submitting}
      destroyOnHidden
      afterOpenChange={(open) => {
        if (open) {
          form.setFieldsValue(initialValues ?? defaultValues);
        }
      }}
    >
      <Form form={form} layout="vertical" size="small">
        <Form.Item
          name="name"
          label={t('form.name')}
          rules={[{ required: true, message: t('form.nameRequired') }]}
        >
          <Input placeholder="web-01" />
        </Form.Item>
        <Form.Item
          name="addr"
          label={t('form.addr')}
          rules={[{ required: true, message: t('form.addrRequired') }]}
        >
          <Input placeholder="10.0.0.1 或 example.com" />
        </Form.Item>
        <Form.Item
          name="port"
          label={t('form.port')}
          rules={[{ required: true, message: t('form.portRequired') }]}
        >
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item
          name="user"
          label={t('form.user')}
          rules={[{ required: true, message: t('form.userRequired') }]}
        >
          <Input placeholder="root" />
        </Form.Item>
        <Form.Item
          name="authType"
          label={t('form.authType')}
          rules={[{ required: true }]}
        >
          <Select
            options={[
              { label: t('form.password'), value: 'password' },
              { label: t('form.privateKey'), value: 'privatekey' },
            ]}
          />
        </Form.Item>
        <Form.Item
          name="secret"
          label={authType === 'privatekey' ? t('form.privateKey') : t('form.password')}
          // 编辑模式 secret 非必填（留空保留原密码），新增必填
          rules={[{ required: !initialValues, message: t('form.secretRequired') }]}
        >
          {authType === 'privatekey' ? (
            <Input.TextArea
              rows={3}
              placeholder={
                initialValues
                  ? t('form.keepSecretPlaceholder')
                  : '-----BEGIN RSA PRIVATE KEY-----...'
              }
            />
          ) : (
            <Input.Password
              placeholder={
                initialValues
                  ? t('form.keepPasswordPlaceholder')
                  : t('form.passwordPlaceholder')
              }
            />
          )}
        </Form.Item>
        <Form.Item name="autoApprove" label="自动放行只读命令">
          <Select
            options={[
              { label: '继承全局设置', value: 'inherit' },
              { label: '允许（覆盖为开启）', value: 'on' },
              { label: '禁止（覆盖为关闭）', value: 'off' },
            ]}
          />
        </Form.Item>
      </Form>
      <div style={{ textAlign: 'right', marginTop: 8 }}>
        <a onClick={handleTest} style={{ fontSize: 12 }}>
          {testing ? t('form.testing') : t('form.testConnection')}
        </a>
      </div>
    </Modal>
  );
}
