import { useCallback, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { Alert, Button, ConfigProvider, Select } from "antd";
import zhCN from "antd/locale/zh_CN";
import { APIError, request, type Project, type User } from "./api";
import Login from "./Login";
import Environments from "./Environments";
import CreateResource from "./CreateResource";
import DesignerPreview from "./DesignerPreview";
import "./style.css";
const sessionKey = "mozi_v2_session";
function readToken() {
  try {
    return sessionStorage.getItem(sessionKey);
  } catch {
    return null;
  }
}
function Shell() {
  const [token, setToken] = useState<string | null>(readToken);
  const [notice, setNotice] = useState("");
  const activeToken = useRef(token);
  const update = useCallback((value: string | null) => {
    activeToken.current = value;
    try {
      if (value) sessionStorage.setItem(sessionKey, value);
      else sessionStorage.removeItem(sessionKey);
    } catch {
      /* Memory-only session when browser storage is unavailable. */
    }
    setToken(value);
  }, []);
  const expire = useCallback(
    (expected: string) => {
      if (activeToken.current !== expected) return;
      setNotice("登录已失效，请重新登录。");
      update(null);
    },
    [update],
  );
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          colorPrimary: "#087a77",
          borderRadius: 8,
          fontFamily:
            'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
        },
      }}
    >
      {token ? (
        <Console
          key={token}
          token={token}
          expire={expire}
          logout={() => {
            setNotice("你已安全退出。");
            update(null);
          }}
        />
      ) : (
        <Login
          notice={notice}
          onLogin={(value) => {
            setNotice("");
            update(value);
          }}
        />
      )}
    </ConfigProvider>
  );
}
function Console({
  token,
  expire,
  logout,
}: {
  token: string;
  expire: (expected: string) => void;
  logout: () => void;
}) {
  const [user, setUser] = useState<User | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [selected, setSelected] = useState("");
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState("");
  const [version, setVersion] = useState(0);
  const [create, setCreate] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const [logoutError, setLogoutError] = useState("");
  const [notice, setNotice] = useState("");
  const preferred = useRef("");
  const [page, setPage] = useState(
    location.hash.split("?")[0] === "#designer" ? "designer" : "environments",
  );
  useEffect(() => {
    const listener = () =>
      setPage(
        location.hash.split("?")[0] === "#designer"
          ? "designer"
          : "environments",
      );
    window.addEventListener("hashchange", listener);
    return () => window.removeEventListener("hashchange", listener);
  }, []);
  const call = useCallback(
    async <T,>(path: string, options: RequestInit = {}): Promise<T> => {
      try {
        return await request<T>(path, token, options);
      } catch (e) {
        if (
          e instanceof APIError &&
          e.status === 401 &&
          !options.signal?.aborted
        )
          expire(token);
        throw e;
      }
    },
    [token, expire],
  );
  useEffect(() => {
    const controller = new AbortController();
    setBusy(true);
    setError("");
    setProjects([]);
    void Promise.all([
      call<User>("/me", { signal: controller.signal }),
      call<Project[]>("/projects", { signal: controller.signal }),
    ])
      .then(([u, p]) => {
        if (controller.signal.aborted) return;
        setUser(u);
        setProjects(p);
        setSelected((old) =>
          p.some((item) => item.id === (preferred.current || old))
            ? preferred.current || old
            : p[0]?.id || "",
        );
        preferred.current = "";
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setBusy(false);
      });
    return () => controller.abort();
  }, [call, version]);
  const project = projects.find((p) => p.id === selected);
  const signOut = async () => {
    setLeaving(true);
    setLogoutError("");
    try {
      await call("/logout", { method: "POST" });
      logout();
    } catch (e) {
      setLogoutError((e as Error).message);
    } finally {
      setLeaving(false);
    }
  };
  return (
    <div className="console">
      <header>
        <strong>
          MOZI <span>/ v2</span>
        </strong>
        <div className="tools">
          <span>{user?.display_name || "验证会话中"}</span>
          <Button onClick={signOut} loading={leaving}>
            退出登录
          </Button>
        </div>
      </header>
      <div className="layout">
        <aside>
          <p>工作空间</p>
          <label htmlFor="project-switch">当前项目</label>
          <Select
            id="project-switch"
            aria-label="当前项目"
            className="project-select"
            value={project?.id}
            loading={busy}
            disabled={busy || !projects.length}
            placeholder="选择项目"
            options={projects.map((p) => ({ value: p.id, label: p.name }))}
            onChange={(id) => {
              setSelected(id);
              setNotice("");
            }}
          />
          <Button
            block
            onClick={() => setCreate(true)}
            disabled={!user || busy}
          >
            新建项目
          </Button>
          <nav aria-label="平台导航">
            <a
              href="#environments"
              aria-current={page === "environments" ? "page" : undefined}
            >
              环境管理
            </a>
            <a
              href="#designer"
              aria-current={page === "designer" ? "page" : undefined}
            >
              模型设计
            </a>
          </nav>
          <div className="sidebar-footer">
            <span className="status-dot" /> Mozi 开发平台
          </div>
        </aside>
        <main>
          {logoutError && <Alert type="error" title={logoutError} showIcon />}
          {notice && <Alert type="success" title={notice} showIcon />}
          {busy ? (
            <section className="summary" role="status">
              正在加载工作空间…
            </section>
          ) : error ? (
            <section className="summary">
              <Alert
                type="error"
                title={error}
                showIcon
                action={
                  <Button onClick={() => setVersion((v) => v + 1)}>重试</Button>
                }
              />
            </section>
          ) : !project ? (
            <section className="summary empty-state">
              <div className="eyebrow">YOUR WORKSPACE</div>
              <h1>从第一个项目开始</h1>
              <p>创建项目后，即可管理团队的开发、预发布与生产环境。</p>
              <Button type="primary" onClick={() => setCreate(true)}>
                创建第一个项目
              </Button>
            </section>
          ) : (
            <>
              <div className="breadcrumb">
                工作空间 / {project.name}
                <span className="role-label">
                  {
                    {
                      owner: "所有者",
                      maintainer: "维护者",
                      developer: "开发者",
                      viewer: "观察者",
                    }[project.role]
                  }
                </span>
              </div>
              {page === "designer" ? (
                <DesignerPreview key={project.id} />
              ) : (
                <Environments key={project.id} project={project} call={call} />
              )}
            </>
          )}
          {create && (
            <CreateResource
              onClose={() => setCreate(false)}
              onSave={async (input) => {
                const data = await call<{ id: string }>("/projects", {
                  method: "POST",
                  body: JSON.stringify({ name: input.name, slug: input.slug }),
                });
                preferred.current = data.id;
                setNotice("项目已创建。");
                setVersion((v) => v + 1);
              }}
            />
          )}
        </main>
      </div>
    </div>
  );
}
createRoot(document.getElementById("root")!).render(<Shell />);
