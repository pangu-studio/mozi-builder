# Mozi v2 platform

阶段 0 集成实验。平台架构及阶段定义见 [architecture.md](../docs/v2/architecture.md)，证据与未完成项见 [progress.md](../docs/v2/progress.md)。暂不支持 Kubernetes。

## 隔离与范围

- 独立 Go module，不升级根模块依赖。go-zero 间接依赖中的 k8s 包来自框架本身，并不代表平台提供 Kubernetes 功能。
- Compose 项目名固定为 `mozi-v2-poc`，使用专属命名卷。仅网关 19080 和 Dkron 18081 发布到 `127.0.0.1`；etcd、RPC、HTTP 及 APISIX Admin API 仅容器网络可见。
- 不读取仓库 `.env`，不连接 PostgreSQL，不创建/导入业务模型。APISIX 配置中的 key 是一次性本地实验凭证，不得用于部署生产。
- RPC 实验复用 go-zero 内置 gRPC Health 协议，验证服务发现及调用链；自定义业务契约在阶段 3 实现。
- HTTP 任务状态为内存 fixture，只验证单实例重复请求，不保证重启/多副本幂等。阶段 5 必须使用持久化逻辑执行 ID。
- 实验的任务 header 使用测试生成的固定 ID；Cron 测试只观测首次触发，不证明每个调度周期都能自动生成新 ID。生产协议需提供“同次重试共享、不同触发不同”的标识。

## 运行 Compose 实验

要求 Go 1.26.4、Docker Engine、Docker Compose、Python 3。构建脚本查询 Docker 服务端架构，编译纯 Go Linux 二进制并构建 scratch 镜像；不要直接 `compose up --build` 跳过本地交叉编译。

从仓库根目录运行：

```bash
make v2-poc-up
make v2-poc-smoke
make v2-poc-down
```

本机单独安装 `docker-compose` 时使用 `make COMPOSE=docker-compose v2-poc-up`。测试脚本自动识别两种 Compose 命令。

如使用独立 Colima profile，先 `colima start --profile mozi-v2`，再给上述命令设置 `DOCKER_CONTEXT=colima-mozi-v2`。所有 Docker 命令沿用显式选择的 context，不需要修改默认 context。

`v2-poc-smoke` 会扩缩容 API、停止/重启注册中心、重启 Controller、创建并删除测试任务。请只针对这个隔离实验运行；finally 会恢复注册中心、Controller 及单 API 实例。`down` 保留卷，不自动删除数据。

## 运行前端实验

```bash
cd platform/web
npm ci
npm run build
npm run preview
```

打开 http://127.0.0.1:15170 。基座使用 hash 导航，切换标签保留设计器工作区，显式卸载按钮才销毁子应用；子应用是独立 HTML 入口、iframe 沙箱、MemoryRouter，复用原有 Guide 页面。模型 CRUD、子应用深链接/生产 SSO、跨项目缓存隔离和完整独立构建发布在后续阶段实现。

浏览器验证（先停止手动 preview，测试会启动独立服务器）：

```bash
# 使用本机 Chrome；每次创建隔离的浏览器上下文
PLAYWRIGHT_CHANNEL=chrome npm run test:e2e
# 或安装 Playwright Chromium 后执行
npx playwright install chromium
npm run test:e2e
```

`npm run dev` 提供 Vite 开发入口；热更新链路应独立验证，生产构建测试不代表热更新通过。

## 数据库接入预备

`internal/config.LoadDatabases` 要求显式 URL，设计库路径只能是 `mozi_v2_design`，平台库只能是 `mozi_v2_platform`，禁止通过查询参数覆盖目标库。错误信息不包含凭证。

阶段 1 使用独立的迁移 CLI。它按数据库分别获取 PostgreSQL advisory lock，每个版本在独立事务中执行，并在 `schema_migrations` 中保存名称、SHA-256 校验和与应用时间。已经应用的 SQL 不得修改；服务启动只运行 `-verify`，不自动迁移。

```bash
make v2-migrate
make v2-migrate-verify
```

CLI 只从进程环境或显式 `-env-file` 读取 `MOZI_DB` 与 `MOZI_PLATFORM_DB`，忽略文件中的其他变量。根目录本地 `.env` 包含两个受限账号，必须保持 Git 忽略。

首批设计库表为 `design_projects`。首批平台库表为 `users`、`projects`、`project_members`、`environments` 和 append-only 语义的 `audit_events`。跨库使用相同项目 ID，不建立外键或跨库事务。认证、项目 API 和应用角色的数据库授权仍在后续变更中实现。

## 测试

```bash
# v1 regression
make test
# v2 unit tests, including unsafe DB configuration and gateway failure handling
make v2-test
# v2 race checks
cd platform && go test -race ./...
```

## 固定版本与来源

- go-zero 1.10.3、etcd client/server 3.5.21、gRPC 1.80.0：`go.mod` / `go.sum`。
- APISIX 3.18.0-debian、Dkron 4.1.3：`deploy/poc/compose.yaml`。
- micro-app 1.0.0-rc.32、React 19.2.7、Ant Design 6.4.5、React Router 7.18.3、Vite 7.3.6：`web/package.json` / `package-lock.json`。
- micro-app 当前使用预发布版本；必须保留生命周期回归用例，升级时重跑。其他组件也不得使用浮动 latest。

官方依据：[go-zero discovery](https://go-zero.dev/guides/microservice/service-discovery/)、[APISIX Admin API](https://apisix.apache.org/docs/apisix/admin-api/)、[Dkron](https://dkron.io/docs/basics/getting-started/)、[micro-app Vite 接入](https://github.com/jd-opensource/micro-app/blob/master/docs/zh-cn/framework/vite.md)。
