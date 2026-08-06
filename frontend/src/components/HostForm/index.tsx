import { Modal, Form, Input, InputNumber, Select, message } from "antd";
import { useState } from "react";
import type { hoststore } from "@wailsjs/go/models";

type HostInput = hoststore.HostInput;

interface HostFormProps {
  open: boolean;
  initialValues?: HostInput | null;
  onCancel: () => void;
  onSubmit: (input: HostInput) => Promise<void>;
  onTest: (input: HostInput) => Promise<boolean>;
}

const defaultValues: HostInput = {
  name: "",
  parentId: "",
  addr: "",
  port: 22,
  user: "root",
  authType: "password",
  secret: "",
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
  const authType = Form.useWatch("authType", form);

  const handleSubmit = async (): Promise<void> => {
    const values = await form.validateFields();
    setSubmitting(true);
    try {
      await onSubmit(values);
      message.success("保存成功");
      onCancel();
    } catch (e) {
      message.error(`保存失败: ${e}`);
    } finally {
      setSubmitting(false);
    }
  };

  const handleTest = async (): Promise<void> => {
    const values = await form.validateFields();
    // 编辑模式：secret 留空表示不修改，测试连接需要真实凭据，提示先输入
    if (initialValues && !values.secret) {
      message.warning("请输入新密码/私钥后再测试");
      return;
    }
    setTesting(true);
    try {
      const ok = await onTest(values);
      if (ok) {
        message.success("连接成功");
      } else {
        message.warning("连接失败");
      }
    } catch (e) {
      message.error(`测试失败: ${e}`);
    } finally {
      setTesting(false);
    }
  };

  return (
    <Modal
      title={initialValues ? "编辑主机" : "添加主机"}
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
          label="名称"
          rules={[{ required: true, message: "请输入主机名称" }]}
        >
          <Input placeholder="web-01" />
        </Form.Item>
        <Form.Item
          name="addr"
          label="地址"
          rules={[{ required: true, message: "请输入 IP 或域名" }]}
        >
          <Input placeholder="10.0.0.1 或 example.com" />
        </Form.Item>
        <Form.Item
          name="port"
          label="端口"
          rules={[{ required: true, message: "请输入端口" }]}
        >
          <InputNumber min={1} max={65535} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item
          name="user"
          label="用户名"
          rules={[{ required: true, message: "请输入用户名" }]}
        >
          <Input placeholder="root" />
        </Form.Item>
        <Form.Item
          name="authType"
          label="认证方式"
          rules={[{ required: true }]}
        >
          <Select
            options={[
              { label: "密码", value: "password" },
              { label: "私钥", value: "privatekey" },
            ]}
          />
        </Form.Item>
        <Form.Item
          name="secret"
          label={authType === "privatekey" ? "私钥 (PEM)" : "密码"}
          // 编辑模式 secret 非必填（留空保留原密码），新增必填
          rules={[{ required: !initialValues, message: "请输入" }]}
        >
          <Input.TextArea
            rows={3}
            placeholder={
              initialValues
                ? "留空则不修改密码/私钥"
                : authType === "privatekey"
                  ? "-----BEGIN RSA PRIVATE KEY-----..."
                  : "输入密码"
            }
          />
        </Form.Item>
      </Form>
      <div style={{ textAlign: "right", marginTop: 8 }}>
        <a onClick={handleTest} style={{ fontSize: 12 }}>
          {testing ? "测试中..." : "测试连接"}
        </a>
      </div>
    </Modal>
  );
}
