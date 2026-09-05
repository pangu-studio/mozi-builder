import { useEffect, useRef, useState } from "react";
import microApp from "@micro-zoe/micro-app";
import type { Project } from "./api";
import type { Call } from "./Environments";
import { Button } from "antd";
microApp.start();
let teardown: Promise<boolean> = Promise.resolve(true);
export default function DesignerPreview({
  project,
  call,
  onDirty,
}: {
  project: Project;
  call: Call;
  onDirty: (dirty: boolean) => void;
}) {
  const slot = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState("正在加载设计器…");
  const [version, setVersion] = useState(0);
  useEffect(() => {
    let cancelled = false;
    const lifetime = new AbortController();
    let app: HTMLElement | undefined;
    setStatus("正在加载设计器…");
    const loaded = () => setStatus("设计器已挂载");
    const failed = () => setStatus("设计器加载失败");
    void teardown.then(() => {
      if (cancelled || !slot.current) return;
      microApp.setData("mozi-console-designer", {
        protocolVersion: 2,
        onDirty,
        project: { id: project.id, name: project.name, role: project.role },
        request: <T,>(path: string, options: RequestInit = {}) => {
          if (
            lifetime.signal.aborted ||
            !/^\/models(?:\/[a-z][a-z0-9_]{0,62}\/[A-Z][A-Za-z0-9]{0,62}(?:\/history)?)?$/.test(
              path,
            )
          )
            return Promise.reject(
              new Error("项目上下文已失效或请求路径不合法。"),
            );
          return call<T>(
            `/projects/${encodeURIComponent(project.id)}/design${path}`,
            {
              ...options,
              signal: AbortSignal.any([
                lifetime.signal,
                ...(options.signal ? [options.signal] : []),
              ]),
            },
          );
        },
      });
      app = document.createElement("micro-app");
      app.setAttribute("name", "mozi-console-designer");
      app.setAttribute(
        "url",
        new URL("/designer/index.html", location.origin).href,
      );
      app.setAttribute("iframe", "");
      app.setAttribute("destroy", "");
      app.addEventListener("mounted", loaded);
      app.addEventListener("error", failed);
      slot.current.appendChild(app);
    });
    return () => {
      cancelled = true;
      lifetime.abort();
      microApp.clearData("mozi-console-designer");
      if (app) {
        app.removeEventListener("mounted", loaded);
        app.removeEventListener("error", failed);
        teardown = microApp.unmountApp("mozi-console-designer", {
          destroy: true,
        });
        app.remove();
      }
    };
  }, [version, project, call, onDirty]);
  return (
    <>
      <section className="summary">
        <h1>模型设计</h1>
        <div className="tools">
          <span role="status">{status}</span>
          {status === "设计器加载失败" && (
            <Button onClick={() => setVersion((v) => v + 1)}>重新加载</Button>
          )}
        </div>
      </section>
      <div ref={slot} />
    </>
  );
}
