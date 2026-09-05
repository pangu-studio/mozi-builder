import { useEffect, useState } from "react";
import { Alert, Button, Table } from "antd";
import type { Environment, Project } from "./api";
import CreateResource from "./CreateResource";
export type Call = <T>(path: string, options?: RequestInit) => Promise<T>;
const kinds = {
  development: "开发环境",
  staging: "预发布环境",
  production: "生产环境",
};
export default function Environments({
  project,
  call,
}: {
  project: Project;
  call: Call;
}) {
  const [rows, setRows] = useState<Environment[]>([]);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState("");
  const [version, setVersion] = useState(0);
  const [open, setOpen] = useState(false);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    setBusy(true);
    setError("");
    setRows([]);
    void call<Environment[]>(
      `/projects/${encodeURIComponent(project.id)}/environments`,
      { signal: controller.signal },
    )
      .then((data) => {
        if (!controller.signal.aborted) setRows(data);
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setBusy(false);
      });
    return () => controller.abort();
  }, [project.id, call, version]);
  const writable = project.role === "owner" || project.role === "maintainer";
  return (
    <>
      <section className="summary">
        <div className="section-heading">
          <div>
            <div className="eyebrow">PROJECT / ENVIRONMENTS</div>
            <h1>环境管理</h1>
            <p>{project.name} · 管理开发、预发布和生产环境。</p>
          </div>
          <div className="tools">
            <Button onClick={() => setVersion((v) => v + 1)} loading={busy}>
              刷新
            </Button>
            {writable && (
              <Button type="primary" onClick={() => setOpen(true)}>
                新建环境
              </Button>
            )}
          </div>
        </div>
        {!writable && (
          <p className="muted">
            当前角色为只读，可联系项目所有者或维护者创建环境。
          </p>
        )}
        {notice && <Alert type="success" title={notice} showIcon />}
        {error ? (
          <Alert
            type="error"
            title={error}
            showIcon
            action={
              <Button onClick={() => setVersion((v) => v + 1)}>重试</Button>
            }
          />
        ) : (
          <Table<Environment>
            rowKey="id"
            loading={busy}
            dataSource={rows}
            pagination={{
              pageSize: 10,
              showSizeChanger: false,
              hideOnSinglePage: true,
            }}
            scroll={{ x: 550 }}
            locale={{ emptyText: busy ? "正在加载环境…" : "暂无环境" }}
            columns={[
              { title: "环境名称", dataIndex: "name" },
              { title: "标识", dataIndex: "slug" },
              {
                title: "类型",
                dataIndex: "kind",
                render: (value: Environment["kind"]) => (
                  <span className={`kind kind-${value}`}>{kinds[value]}</span>
                ),
              },
              {
                title: "保护状态",
                dataIndex: "kind",
                render: (value: string) =>
                  value === "production" ? "受保护" : "普通环境",
              },
            ]}
          />
        )}
      </section>
      {open && (
        <CreateResource
          environment
          onClose={() => setOpen(false)}
          onSave={async (input) => {
            await call(
              `/projects/${encodeURIComponent(project.id)}/environments`,
              { method: "POST", body: JSON.stringify(input) },
            );
            setNotice("环境已创建。");
            setVersion((v) => v + 1);
          }}
        />
      )}
    </>
  );
}
