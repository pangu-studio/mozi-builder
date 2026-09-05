import { useState } from "react";
import { Alert, Button, Form, Input, Modal, Select } from "antd";
export type ResourceInput = { name: string; slug: string; kind?: string };
export default function CreateResource({
  environment = false,
  onClose,
  onSave,
}: {
  environment?: boolean;
  onClose: () => void;
  onSave: (input: ResourceInput) => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const submit = async (values: ResourceInput) => {
    setBusy(true);
    setError("");
    try {
      await onSave(values);
      onClose();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <Modal
      title={environment ? "新建环境" : "新建项目"}
      open
      onCancel={() => {
        if (!busy) onClose();
      }}
      closable={!busy}
      mask={{ closable: !busy }}
      keyboard={!busy}
      footer={null}
      destroyOnHidden
    >
      <Form
        layout="vertical"
        onFinish={submit}
        initialValues={{ kind: "development" }}
        disabled={busy}
        requiredMark={false}
      >
        {error && <Alert type="error" showIcon title={error} />}
        <Form.Item
          label="名称"
          name="name"
          rules={[
            { required: true, whitespace: true, message: "请输入名称" },
            { max: 60, message: "名称最多 60 个字符" },
          ]}
        >
          <Input maxLength={60} autoFocus />
        </Form.Item>
        <Form.Item
          label="标识"
          name="slug"
          extra="2–63 位小写字母、数字或连字符，以字母开头。"
          rules={[
            { required: true, message: "请输入标识" },
            {
              pattern: /^[a-z][a-z0-9-]{1,62}$/,
              message: "请使用有效标识，例如 my-project",
            },
          ]}
        >
          <Input
            maxLength={63}
            placeholder={environment ? "development" : "my-project"}
          />
        </Form.Item>
        {environment && (
          <Form.Item label="环境类型" name="kind">
            <Select
              options={[
                { value: "development", label: "开发环境" },
                { value: "staging", label: "预发布环境" },
                { value: "production", label: "生产环境" },
              ]}
            />
          </Form.Item>
        )}
        {environment && (
          <p className="muted">生产环境会自动标记为受保护环境。</p>
        )}
        <div className="tools form-actions">
          <Button disabled={busy} onClick={onClose}>
            取消
          </Button>
          <Button type="primary" htmlType="submit" loading={busy}>
            创建
          </Button>
        </div>
      </Form>
    </Modal>
  );
}
