import { useEffect, useState } from "react";
import { Alert, Button, ConfigProvider, Form, Input, Modal, Table } from "antd";
import FieldTable from "../../../builder-react/components/FieldTable";
import FieldEditor from "../../../builder-react/components/FieldEditor";
import type { FieldIR } from "../../../builder-react/api/dev-platform";

type Document = {
  schema_version?: number;
  module: string;
  model: string;
  label: string;
  table: string;
  description?: string;
  fields: FieldIR[];
  [key: string]: unknown;
};
type Model = {
  module: string;
  name: string;
  version: string;
  document: Document;
};
type History = {
  version: string;
  document: Document;
  action: string;
  actor_id: string;
  created_at: string;
};
export type DesignContext = {
  protocolVersion: 2;
  onDirty: (dirty: boolean) => void;
  project: { id: string; name: string; role: string };
  request: <T>(path: string, options?: RequestInit) => Promise<T>;
};
const fresh = (): Document => ({
  schema_version: 1,
  module: "content",
  model: "",
  label: "",
  table: "",
  fields: [
    {
      name: "id",
      type: "string",
      label: "ID",
      primary: true,
      required: true,
      generated: "uuid",
    },
  ],
  relations: [],
  semantics: {},
  admin: {},
  ui_intent: {},
  api_intent: {},
});
const path = (m: Model) =>
  `/models/${encodeURIComponent(m.module)}/${encodeURIComponent(m.name)}`;
export default function ModelsWorkspace({
  context,
}: {
  context: DesignContext;
}) {
  const [models, setModels] = useState<Model[]>([]);
  const [selected, setSelected] = useState<Model | null>(null);
  const [draft, setDraft] = useState<Document | null>(null);
  const [extra, setExtra] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [revision, setRevision] = useState(0);
  const [editor, setEditor] = useState<number | null>(null);
  const [history, setHistory] = useState<History[] | null>(null);
  const [dirty, setDirty] = useState(false);
  const [deleting, setDeleting] = useState(false);
  useEffect(() => {
    context.onDirty(dirty);
    return () => context.onDirty(false);
  }, [dirty, context]);
  const writable = ["owner", "maintainer", "developer"].includes(
    context.project.role,
  );
  const load = (model: Model | null) => {
    const d = model ? structuredClone(model.document) : fresh();
    setSelected(model);
    setDraft(d);
    const {
      module: _,
      model: __,
      label: ___,
      table: ____,
      description: _____,
      fields: ______,
      ...rest
    } = d;
    setExtra(JSON.stringify(rest, null, 2));
    setDirty(false);
    setError("");
  };
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    void context
      .request<Model[]>("/models", { signal: controller.signal })
      .then((data) => {
        if (!controller.signal.aborted) setModels(data);
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [context, revision]);
  useEffect(() => {
    const listener = (e: BeforeUnloadEvent) => {
      if (dirty) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", listener);
    return () => window.removeEventListener("beforeunload", listener);
  }, [dirty]);
  const open = async (model: Model) => {
    if (dirty && !window.confirm("放弃尚未保存的修改并读取模型？")) return;
    setBusy(true);
    try {
      load(await context.request<Model>(path(model)));
    } catch (e) {
      setError(
        (e as { status?: number }).status === 409
          ? "模型版本已变更，请重新载入最新版本。"
          : (e as Error).message,
      );
    } finally {
      setBusy(false);
    }
  };
  const save = async () => {
    if (!draft) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const metadata = JSON.parse(extra);
      if (!metadata || typeof metadata !== "object" || Array.isArray(metadata))
        throw new Error("扩展定义必须为 JSON 对象。");
      const document = {
        ...metadata,
        module: draft.module,
        model: draft.model,
        label: draft.label,
        table: draft.table,
        description: draft.description,
        fields: draft.fields,
      };
      const result = await context.request<Model>(
        selected ? path(selected) : "/models",
        {
          method: selected ? "PUT" : "POST",
          body: JSON.stringify({ version: selected?.version, document }),
        },
      );
      load(result);
      setNotice("模型已保存，历史版本已记录。");
      setRevision((v) => v + 1);
    } catch (e) {
      setError(
        (e as { status?: number }).status === 409
          ? "保存冲突：模型已被修改或同名模型已存在。本地编辑内容已保留，请核对最新版本后再保存。"
          : (e as Error).message,
      );
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="designer" data-testid="real-designer">
      <div className="section-heading">
        <div>
          <div className="eyebrow">{context.project.name}</div>
          <h1>项目模型</h1>
          <p className="muted">
            模型定义保存在项目设计库中，每次保存均生成独立版本。
          </p>
        </div>
        <div className="tools">
          <Button onClick={() => setRevision((v) => v + 1)} loading={loading}>
            刷新列表
          </Button>
          {writable && (
            <Button
              type="primary"
              onClick={() => {
                if (!dirty || window.confirm("放弃尚未保存的修改？"))
                  load(null);
              }}
            >
              新建模型
            </Button>
          )}
        </div>
      </div>
      {error && <Alert type="error" title={error} showIcon />}
      {notice && <Alert type="success" title={notice} showIcon />}
      <Table<Model>
        rowKey={(m) => `${m.module}/${m.name}`}
        loading={loading}
        dataSource={models}
        pagination={{ pageSize: 10, showSizeChanger: false }}
        locale={{ emptyText: "本项目暂无模型" }}
        columns={[
          { title: "模块", dataIndex: "module" },
          {
            title: "模型",
            render: (_, m) => (
              <Button type="link" onClick={() => void open(m)}>
                {m.name}
              </Button>
            ),
          },
          { title: "名称", render: (_, m) => m.document.label },
          { title: "数据表", render: (_, m) => m.document.table },
        ]}
      />
      {draft && (
        <section className="summary">
          <h2>
            {selected ? `${selected.module} / ${selected.name}` : "新建模型"}
          </h2>
          <ConfigProvider componentDisabled={busy || !writable}>
            <Form layout="vertical">
              <div className="model-metadata">
                {(["module", "model", "label", "table"] as const).map(
                  (key, i) => (
                    <Form.Item
                      key={key}
                      label={
                        [
                          "模块标识",
                          "模型标识（PascalCase）",
                          "模型名称",
                          "数据表名",
                        ][i]
                      }
                    >
                      <Input
                        aria-label={
                          ["模块标识", "模型标识", "模型名称", "数据表名"][i]
                        }
                        value={draft[key]}
                        disabled={
                          !writable ||
                          busy ||
                          (!!selected && (key === "module" || key === "model"))
                        }
                        onChange={(e) => {
                          setDraft({ ...draft, [key]: e.target.value });
                          setDirty(true);
                        }}
                      />
                    </Form.Item>
                  ),
                )}
              </div>
              <Form.Item label="描述">
                <Input.TextArea
                  value={draft.description || ""}
                  onChange={(e) => {
                    setDraft({ ...draft, description: e.target.value });
                    setDirty(true);
                  }}
                />
              </Form.Item>
            </Form>
            <FieldTable
              fields={draft.fields}
              onAdd={() => setEditor(-1)}
              onEdit={(_, i) => setEditor(i)}
              onDelete={(_, i) => {
                setDraft({
                  ...draft,
                  fields: draft.fields.filter((_, j) => j !== i),
                });
                setDirty(true);
              }}
              onMove={(from, to) => {
                const fields = [...draft.fields];
                const [field] = fields.splice(from, 1);
                fields.splice(to, 0, field);
                setDraft({ ...draft, fields });
                setDirty(true);
              }}
            />
            <Form layout="vertical">
              <Form.Item
                label="领域语义、管理后台、产品 UI、API 契约与关系（JSON）"
                extra="保留已有键，完整编辑 semantics、admin、ui_intent、api_intent、relations 等定义。"
              >
                <Input.TextArea
                  aria-label="扩展定义"
                  rows={12}
                  value={extra}
                  onChange={(e) => {
                    setExtra(e.target.value);
                    setDirty(true);
                  }}
                />
              </Form.Item>
            </Form>
          </ConfigProvider>
          <div className="tools">
            {writable && (
              <Button
                type="primary"
                aria-label="保存模型"
                loading={busy}
                onClick={() => void save()}
              >
                保存模型
              </Button>
            )}
            {selected && (
              <>
                <Button disabled={busy} onClick={() => void open(selected)}>
                  重新载入最新版本
                </Button>
                <Button
                  disabled={busy}
                  onClick={async () => {
                    try {
                      setHistory(
                        await context.request<History[]>(
                          path(selected) + "/history",
                        ),
                      );
                    } catch (e) {
                      setError(
                        (e as { status?: number }).status === 409
                          ? "模型版本已变更，请重新载入最新版本。"
                          : (e as Error).message,
                      );
                    }
                  }}
                >
                  版本历史
                </Button>
                {writable && (
                  <Button
                    danger
                    disabled={busy}
                    onClick={() => setDeleting(true)}
                  >
                    删除模型
                  </Button>
                )}
              </>
            )}
            <span className="muted">
              {dirty ? "尚有未保存修改" : selected ? "已保存" : "尚未保存"}
            </span>
          </div>
          {editor !== null && (
            <FieldEditor
              visible
              field={editor < 0 ? null : draft.fields[editor]}
              onCancel={() => setEditor(null)}
              onOk={(field) => {
                const fields = [...draft.fields];
                if (editor < 0) fields.push(field);
                else fields[editor] = { ...fields[editor], ...field };
                setDraft({ ...draft, fields });
                setDirty(true);
                setEditor(null);
              }}
            />
          )}
        </section>
      )}
      <Modal
        title="模型历史"
        open={history !== null}
        footer={null}
        onCancel={() => setHistory(null)}
        width={800}
      >
        {history?.map((h) => (
          <details key={h.version}>
            <summary>
              {new Date(h.created_at).toLocaleString()} · {h.action} ·{" "}
              {h.version}
            </summary>
            <pre className="model-snapshot">
              {JSON.stringify(h.document, null, 2)}
            </pre>
          </details>
        ))}
      </Modal>
      <Modal
        title="删除当前模型？"
        open={deleting}
        confirmLoading={busy}
        onCancel={() => {
          if (!busy) setDeleting(false);
        }}
        onOk={async () => {
          if (!selected) return;
          setBusy(true);
          try {
            await context.request(path(selected), {
              method: "DELETE",
              body: JSON.stringify({ version: selected.version }),
            });
            setDraft(null);
            setSelected(null);
            setDirty(false);
            setDeleting(false);
            setRevision((v) => v + 1);
          } catch (e) {
            setError(
              (e as { status?: number }).status === 409
                ? "模型版本已变更，请重新载入最新版本。"
                : (e as Error).message,
            );
            setDeleting(false);
          } finally {
            setBusy(false);
          }
        }}
      >
        <p>当前版本将被标记为删除，历史记录保留。同名模型暂不可重新创建。</p>
      </Modal>
    </div>
  );
}
