import React, { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import microApp from "@micro-zoe/micro-app";
import { Button, ConfigProvider, theme } from "antd";
import zhCN from "antd/locale/zh_CN";
import "./style.css";

microApp.start();
function Shell() {
  const [page, setPage] = useState(
    location.hash === "#operations" ? "operations" : "designer",
  );
  const [mounted, setMounted] = useState(true);
  const [dark, setDark] = useState(false);
  const [status, setStatus] = useState("等待加载");
  const slot = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const update = () =>
      setPage(location.hash === "#operations" ? "operations" : "designer");
    window.addEventListener("hashchange", update);
    return () => window.removeEventListener("hashchange", update);
  }, []);
  // Preserve the designer workspace while switching base tabs. Explicit unload
  // waits for teardown before reusing the globally unique micro-app name.
  const teardown = useRef<Promise<boolean>>(Promise.resolve(true));
  useEffect(() => {
    if (!mounted) return;
    let cancelled = false;
    let app: HTMLElement | undefined;
    const loaded = () => setStatus("设计器已挂载");
    const failed = () => setStatus("设计器加载失败");
    setStatus("加载设计器中");
    void teardown.current.then(() => {
      if (cancelled || !slot.current) return;
      app = document.createElement("micro-app");
      app.setAttribute("name", "mozi-designer");
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
        teardown.current = microApp.unmountApp("mozi-designer", {
          destroy: true,
        });
        app.remove();
      }
    };
  }, [mounted]);
  useEffect(() => {
    microApp.setData("mozi-designer", {
      protocolVersion: 1,
      theme: dark ? "dark" : "light",
    });
  }, [dark, page, mounted]);
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: { colorPrimary: "#087a77" },
      }}
    >
      <div className={dark ? "dark" : ""}>
        <header>
          <strong>
            MOZI <span> / v2</span>
          </strong>
          <span>开发平台 · 集成验证</span>
        </header>
        <div className="layout">
          <aside>
            <p>工作空间 / 技术预览</p>
            <nav>
              <a
                href="#designer"
                aria-current={page === "designer" ? "page" : undefined}
              >
                模型设计
              </a>
              <a
                href="#operations"
                aria-current={page === "operations" ? "page" : undefined}
              >
                运行管理
              </a>
            </nav>
          </aside>
          <main>
            <section className="summary">
              <div className="eyebrow">PHASE 0 / INTEGRATION LAB</div>
              <h1>{page === "designer" ? "模型设计工作台" : "运行管理集成"}</h1>
              <p>
                验证独立子应用的加载、路由和主题。本页面使用演示数据，不连接设计数据库。
              </p>
              <div className="tools">
                <Button onClick={() => setDark(!dark)}>切换主题</Button>
                {page === "designer" && (
                  <Button onClick={() => setMounted(!mounted)}>
                    {mounted ? "卸载设计器" : "挂载设计器"}
                  </Button>
                )}
                {page === "designer" && (
                  <span className="status" role="status">
                    {mounted ? status : "设计器已卸载"}
                  </span>
                )}
              </div>
            </section>
            <div ref={slot} hidden={page !== "designer"} />
            {page === "operations" && (
              <section className="summary">
                <h2>运行链路</h2>
                <p>API 网关 → HTTP 服务 → RPC 服务</p>
                <p>注册发现：etcd · API 网关：APISIX · 定时任务：Dkron</p>
                <p>服务状态由 Compose 集成测试验证，此页尚未接入运行数据。</p>
              </section>
            )}
          </main>
        </div>
      </div>
    </ConfigProvider>
  );
}
createRoot(document.getElementById("root")!).render(<Shell />);
