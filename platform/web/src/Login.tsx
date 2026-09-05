import { useState } from "react";
import { Alert, Button, Form, Input } from "antd";
import { APIError, request } from "./api";
export default function Login({
  onLogin,
  notice,
}: {
  onLogin: (token: string) => void;
  notice: string;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const submit = async (values: { email: string; password: string }) => {
    setBusy(true);
    setError("");
    try {
      const data = await request<{ access_token: string }>("/login", null, {
        method: "POST",
        body: JSON.stringify(values),
      });
      onLogin(data.access_token);
    } catch (e) {
      setError(
        e instanceof APIError && e.status === 401
          ? "邮箱或密码不正确。"
          : (e as Error).message,
      );
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="login-layout">
      <section className="login-story">
        <div className="brand">
          MOZI <span>/ v2</span>
        </div>
        <div>
          <div className="eyebrow">DEVELOPMENT PLATFORM</div>
          <h1>
            从模型设计
            <br />
            到服务运行。
          </h1>
          <p>
            连接团队、项目与环境，
            <br />
            让开发工作在一个空间内有序推进。
          </p>
        </div>
        <small>工作空间 · 模型设计 · 环境管理</small>
      </section>
      <main className="login-main">
        <div className="login-card">
          <h2>登录开发平台</h2>
          <p>使用管理员为你创建的账号。</p>
          {(error || notice) && (
            <Alert
              type={error ? "error" : "info"}
              title={error || notice}
              showIcon
            />
          )}
          <Form
            layout="vertical"
            onFinish={submit}
            disabled={busy}
            requiredMark={false}
          >
            <Form.Item
              name="email"
              label="邮箱"
              rules={[
                { required: true, message: "请输入邮箱" },
                { type: "email", message: "请输入有效邮箱" },
              ]}
            >
              <Input
                autoComplete="username"
                maxLength={254}
                placeholder="name@company.com"
                size="large"
              />
            </Form.Item>
            <Form.Item
              name="password"
              label="密码"
              rules={[{ required: true, message: "请输入密码" }]}
            >
              <Input.Password autoComplete="current-password" size="large" />
            </Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              block
              size="large"
              loading={busy}
            >
              登录平台
            </Button>
          </Form>
          <p className="muted">没有账号或忘记密码？请联系平台管理员。</p>
        </div>
      </main>
    </div>
  );
}
