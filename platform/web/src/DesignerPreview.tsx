import { useEffect, useRef, useState } from "react";
import microApp from "@micro-zoe/micro-app";
import { Alert, Button } from "antd";
microApp.start();
let teardown: Promise<boolean> = Promise.resolve(true);
export default function DesignerPreview() {
  const slot = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState("正在加载设计器…");
  const [version, setVersion] = useState(0);
  useEffect(() => {
    let cancelled = false;
    let app: HTMLElement | undefined;
    setStatus("正在加载设计器…");
    const loaded = () => setStatus("设计器已挂载");
    const failed = () => setStatus("设计器加载失败");
    void teardown.then(() => {
      if (cancelled || !slot.current) return;
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
      if (app) {
        app.removeEventListener("mounted", loaded);
        app.removeEventListener("error", failed);
        teardown = microApp.unmountApp("mozi-console-designer", {
          destroy: true,
        });
        app.remove();
      }
    };
  }, [version]);
  return (
    <>
      <section className="summary">
        <h1>模型设计</h1>
        <Alert
          type="info"
          title="设计器预览"
          description="当前展示演示内容；项目模型的读取与保存将在下一阶段接入。"
          showIcon
        />
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
