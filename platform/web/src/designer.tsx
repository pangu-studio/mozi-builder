import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { App, Button, ConfigProvider, Modal, theme } from "antd";
import zhCN from "antd/locale/zh_CN";
import { MemoryRouter, Routes, Route, Link } from "react-router-dom";
import axios from "axios";
import { MoziBuilderProvider } from "../../../builder-react/MoziBuilderProvider";
import Guide from "../../../builder-react/pages/Guide";
import "./style.css";
import ModelsWorkspace, { type DesignContext } from "./ModelsWorkspace";

type Context = {
  protocolVersion?: number;
  theme?: string;
  project?: DesignContext["project"];
  request?: DesignContext["request"];
  onDirty?: DesignContext["onDirty"];
};
const bridge = (
  window as Window & {
    microApp?: {
      addDataListener: (
        listener: (data: Context) => void,
        autoTrigger: boolean,
      ) => void;
      removeDataListener: (listener: (data: Context) => void) => void;
    };
  }
).microApp;
const client = axios.create();
const guide =
  "# Mozi v2 设计器\n\n当前挂载的是现有 builder-react 的 Guide 页面。\n\n- 独立应用生命周期\n- 主题通过版本化上下文传递\n- 原有组件库继续复用\n\n此 PoC 不连接数据库；模型 CRUD 在阶段 2 接入。";
function Probe() {
  const [open, setOpen] = useState(false);
  const { message } = App.useApp();
  return (
    <>
      <h1>设计器集成检查</h1>
      <nav>
        <Link to="/designer/guide">开发指南</Link>
        <Link to="/designer/check">交互检查</Link>
      </nav>
      <Routes>
        <Route path="/designer/guide" element={<Guide />} />
        <Route
          path="/designer/check"
          element={
            <div className="tools">
              <Button onClick={() => setOpen(true)}>打开弹窗</Button>
              <Button onClick={() => message.success("设计器通知正常")}>
                显示通知
              </Button>
            </div>
          }
        />
      </Routes>
      <Modal
        title="子应用弹窗"
        open={open}
        onOk={() => setOpen(false)}
        onCancel={() => setOpen(false)}
        getContainer={() => document.getElementById("designer-root")!}
      >
        <p>弹窗保留在设计器容器内。</p>
      </Modal>
    </>
  );
}
function Designer() {
  const [dark, setDark] = useState(false);
  const [projectContext, setProjectContext] = useState<DesignContext | null>(
    null,
  );
  useEffect(() => {
    const receive = (data: Context) => {
      if (
        data.protocolVersion === 2 &&
        data.project &&
        typeof data.request === "function"
      )
        setProjectContext(data as DesignContext);
      if (data.protocolVersion === 1) setDark(data.theme === "dark");
    };
    bridge?.addDataListener(receive, true);
    return () => bridge?.removeDataListener(receive);
  }, []);
  return (
    <ConfigProvider
      locale={zhCN}
      getPopupContainer={() => document.getElementById("designer-root")!}
      theme={{
        algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: { colorPrimary: "#087a77" },
      }}
    >
      <App
        message={{
          getContainer: () => document.getElementById("designer-root")!,
        }}
      >
        <div
          className={`designer ${dark ? "dark" : ""}`}
          data-testid="designer-app"
        >
          {projectContext ? (
            <ModelsWorkspace context={projectContext} />
          ) : (
            <MemoryRouter initialEntries={["/designer/guide"]}>
              <MoziBuilderProvider
                apiClient={client}
                routeBasePath="/designer"
                guideMarkdown={guide}
              >
                <Probe />
              </MoziBuilderProvider>
            </MemoryRouter>
          )}
        </div>
      </App>
    </ConfigProvider>
  );
}
const root = createRoot(document.getElementById("designer-root")!);
root.render(<Designer />);
window.addEventListener("unmount", () => root.unmount(), { once: true });
