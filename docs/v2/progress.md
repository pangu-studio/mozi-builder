# v2 实施进度

更新：2026-09-05。用户确认 Docker Compose，暂不考虑 Kubernetes。

## 阶段 0：核心集成验证通过

| 验证 | 结果 | 证据 |
|---|---|---|
| 根模块 v1 回归 | 通过 | 根目录 go test ./... |
| go-zero HTTP → etcd 发现的 RPC | 通过 | smoke.py 经 APISIX 请求内置 gRPC Health |
| HTTP 实例扩容及缩容 | 通过 | 从 1 扩到 2，观测不同实例；缩回 1 后持续成功 |
| 注册中心停机 | 通过 | 缓存的网关配置及 RPC 连接继续处理请求 |
| Controller 重启、零实例、新实例 | 通过 | 重启后将 API 缩至 0 得到 404，再扩至 1 恢复 200 |
| Dkron HTTP 重试 | 通过 | 首次返回 503，重试完成；同一配置执行 ID 重复提交不重复生效 |
| Dkron 超时 | 通过 | 1 秒 executor 超时记录失败，业务未完成 |
| Dkron 定时触发 | 通过 | @every 3s 首次触发成功；立即删除实验任务 |
| micro-app + React 19 + Ant Design 6 | 通过 | 原有 Guide 页面挂载、弹窗、通知、主题、卸载重挂、基座后退及刷新 |
| 生命周期回归 | 通过 | 标签切换保留工作区，显式卸载等待销毁；最终连续 5 次通过 |
| npm audit | 通过 | 最终锁文件报告 0 vulnerabilities |
| 前端构建及 antd lint | 通过 | TypeScript、Vite production build；antd lint 无问题 |
| 新库配置防护 | 单元验证通过 | 拒绝空配置、旧库、两库混用、URL 参数覆盖；错误不泄露密码 |

真实端到端运行环境：macOS ARM64、独立 Colima profile `mozi-v2`，Compose 项目 `mozi-v2-poc`；浏览器为隔离上下文中的本机 Chrome。所有实验端口仅发布在回环地址。

## 已解决的集成问题

1. 子应用卸载后立刻浏览器后退曾出现空白。明确基座标签切换保留工作区，仅显式卸载销毁；保留稳定容器并等待异步销毁后再复用名称。
2. Dkron 4.1.3 的禁用任务不能直接通过 run 接口执行。实验手动任务保持启用、使用远期调度；正式平台必须明确定义暂停与“立即执行”的交互和权限。
3. APISIX 零实例时禁用路由，而注册中心连接失败保留最后配置。禁用路由用空 plugins 满足 APISIX schema，不引用尚未创建的上游。
4. 固定 npm 版本后，依赖审计发现的 React Router 与 Vite 高风险项已升级至同版本线修复版本。

## 尚未完成，不属于本次 PoC 的生产能力

- 平台 API、认证/SSO、项目/环境管理、双库迁移、持久化审计及发布操作。
- 模型 CRUD、多项目设计隔离、完整设计器、go-zero 业务契约生成与 ent 产物。
- 子应用深链接与 Vite 热更新验证、独立构建发布、生产登录态。
- Cron 每次触发独立 ID、跨重试稳定 ID 的完整调度协议；当前固定 header 是测试 fixture。
- 跨实例/跨重启任务幂等、长任务、补偿、故障恢复、生产认证和高可用。
- Watch compact/failure 的完整自动故障注入；当前 controller 在 watch 出错后全量重读，但不能据此宣称所有分区场景验证通过。

两个远程 v2 数据库保持空库；本次没有运行任何 PostgreSQL 迁移。

## 下一阶段：平台基础

先实现可审查的版本化迁移与数据库连接校验，再接入 go-zero API、身份与项目/环境权限、审计，最后连接前端基座。不得将 PoC 的内存任务状态或固定实验凭证直接迁入平台运行代码。

### 阶段 1 进展

- 双库迁移 CLI 已实现：独立迁移集合、advisory lock、逐版本事务、SHA-256 漂移检测、只读 verify 模式。
- `0001_projects` 已应用到 `mozi_v2_design`；`0001_identity_projects` 已应用到 `mozi_v2_platform`，随后只读校验通过。
- 平台库当前包含用户、项目、成员角色、环境和审计表；设计库只建立项目作用域。没有导入旧库数据。
- 数据库配置继续拒绝缺失值、旧库名、双库混用及 URL 查询参数覆盖；迁移 CLI 的 `-env-file` 只读取两个数据库键，不通过 shell 解释凭证。
- 此数据库基线交付时尚未接入认证与 API；后续实现见下节。

### 阶段 1：认证与项目 API

- 新增 go-zero 平台 API 与用户创建命令，启动只核验双库状态。
- 随机 Bearer 会话仅存摘要，支持过期、撤销、禁用用户失效；bcrypt 与数据库来源地址限流。
- 项目创建/列表、环境创建/列表、项目成员新增/角色修改/列表已实现，服务端强制角色权限。
- 变更和审计同事务；审计表触发器禁止 UPDATE/DELETE。
- PostgreSQL 独立临时 schema 集成验证覆盖跨项目拒绝、角色边界、会话失效及审计失败回滚。
- `0002_sessions` 已应用到专用平台库，双库只读核验通过。真实 API 启动后匿名 `/me` 返回 401、非法登录 JSON 返回 400；测试进程已停止。
- API 契约与当前范围见 [platform-api.md](platform-api.md)。前端登录接入、完整分页、运行账号分权和后续设计器项目作用域仍待实现。

### 阶段 1：控制台接入

- 默认入口接入登录、项目创建/切换和环境管理，Vite 同源代理连接平台 API。
- 会话失效、网络错误、空项目、重复标识及只读角色均有页面反馈；旧请求的 401 不影响新的登录会话。
- 切换项目取消旧请求并销毁旧环境视图。设计器以 micro-app 继续挂载，hash 解析兼容子应用追加的路由参数。
- 原实验工作台保留在 `/lab.html#designer`；真实项目模型与运行管理子应用在后续阶段接入。
- 验证：TypeScript/Vite 构建、Ant Design lint 与九项 Chrome 浏览器测试通过；桌面和 390px 移动布局已检查。浏览器用例使用模拟 API；另行实测 Vite 到真实 go-zero API 的同源代理，匿名请求返回 401、非法登录 JSON 返回 400。

### 阶段 2：真实项目模型

- 增加设计库项目作用域模型表和不可变版本历史，复用固定版本的 ModelIR 与校验器，未修改 v1 存储。
- 模型列表/读写/删除/历史均强制项目鉴权；更新删除要求版本，条件写入阻止并发覆盖。
- micro-app 子应用接入真实模型，复用字段组件，支持完整扩展定义、冲突保留、历史查看和切换项目销毁旧实例。
- 边界和契约见 [project-models.md](project-models.md)：高级定义目前为 JSON 编辑，模块由模型归属表达，完整模块管理、关系解析及生成流程后续完善。

- 设计库 `0002_models` 已应用，双库 verify 通过，本地 API 已更新。平台竞态测试与 go vet、v1 全量回归、十项浏览器用例、Ant Design lint 与构建均通过。设计器改为独立构建，修复共享分块在 iframe 内偶发停滞。

### 阶段 3：服务契约（PR-A/B）

- 设计契约见 [service-ir.md](service-ir.md)：ServiceIR 与 ModelIR 并列，proto 字段编号显式存储，HTTP 链路优先、proto/RPC 模板排后（PR-E）。
- 根模块新增 ServiceIR 类型、`mozi/service` 校验与编号规则、differ 的 ServiceIR 兼容性分类；纯新增，v1 回归通过。
- 设计库 `0003_services` 新增 design_services / design_service_history，复用模型的乐观版本与不可变历史模式；`design/services` 接口与模型接口共用同一鉴权与版本协议。
- 验证：平台 go test -race 覆盖服务文档的同名隔离、跨项目六类接口拒绝、viewer 只读、428/409、并发仅一次成功、reserved 编号复用拒绝、历史失败回滚与删除保留；双库 verify 通过。控制台服务编辑界面、ChangePlan 接入与示例服务在 PR-C/D/E。

### 阶段 3：ChangePlan 装配与平台接入（PR-D1）

- ChangePlan 装配逻辑从 devplatform 原样抽取到根模块 `mozi/changeplan`（纯函数，无 gin/v1 设计库依赖）；v1 以类型别名 + 薄适配层保持 CLI 与 HTTP 行为不变，回归通过。
- 平台新增 `GET .../design/models/:module/:name/change-plan`：design_models 当前文档对上一历史快照做 differ 对比，装配 AI Coding 契约；v2 无 manifest，状态为 pending / no_diff。
- 验证：真实双库 race 测试覆盖创建后 pending、更新后差异、viewer 可读、跨项目 404、services 集合 404；平台 go test -race 与 go vet 全绿。示例 HTTP 服务（渲染产物可运行）在 PR-D2，依赖模板 PR 合并。

### 阶段 3：示例 HTTP 服务可运行（PR-D2）

- `platform/internal/gen/example/`：fixture ServiceIR（docs/v2/service-ir.md 同款）渲染出 `types_gen.go` 与 `handler_gen.go`，提交产物随 platform 编译；业务逻辑手写在标记外。
- 再生成工作流 `MOZI_REGEN_EXAMPLE=1 go test ./internal/gen/example/ -run TestRegenerate`：types 全量重写，handler 经 `generator.SpliceRendered` 只替换标记段；测试断言已提交产物与新渲染恒等（手写逻辑保留）。
- `TestExampleServesHTTP` 用 httptest 起真实服务：health/list/create 三个端点 200 且响应正确，畸形 JSON 被生成的解析装配拒绝。
- 附带修复：marker 拼接幂等性（SpliceRendered dedent）与空 section 空白行，见根模块 PR。
- 阶段 3 剩余：proto/RPC 模板与 RPC 示例（PR-E）；阶段退出条件中 HTTP 部分已达成。
### 阶段 3：HTTP 生成模板（PR-C）

- 根模块新增 `ServiceTemplateContext` 与 `generator.ExecuteService`；模板 `service/api.tmpl` 渲染 go-zero .api（消息类型、jwt 分组与 public 分组），`service/handler.go.tmpl` 渲染 handler 骨架，请求装配在 `mozi:section` 标记内、业务逻辑在标记外。
- ent schema 复用既有 `backend/schema.go.tmpl`（ModelIR → ent），无重复模板。
- 验证：golden 文件测试（`-update` 刷新）、marker 抽取/替换/追加单测、增量再生成保留手写业务逻辑的端到端测试；根模块 go test 与 go vet 通过。ChangePlan 接入与示例服务运行在 PR-D。

### 阶段 3：RPC 示例可运行（PR-E）

- `service/proto.tmpl` 渲染 proto3：字段编号原样取自 IR、删除字段 reserved 透传、RPC service 块；`ProtoType` 映射 int→int64、time→int64（unix 毫秒）等集中在 `ServiceFieldContext`。
- `platform/internal/gen/rpcexample/`：渲染产物经纯 Go protocompile 解析验证（字段编号、reserved 编号与名称、service 方法），并用 bufconn + JSON codec 完成真实 gRPC 往返（`/content.ContentService/GetDeck`）。
- 验证：平台 go test -race 全绿。示例用手写 ServiceDesc + JSON codec，未引入 protoc 代码生成；生产 goctl 工具链接入在发布阶段（阶段 6）评估。

### 阶段 4：发布基础（PR-A）

- 设计契约见 [deployment.md](deployment.md)：发布状态机、双适配器红线、幂等与恢复语义、与 PoC fixture 的边界。
- 平台库 `0003_releases`：`release_operations`（idempotency_key 唯一、活动操作部分唯一索引保证单一配置管理者）+ `release_readbacks`（append-only 回读快照）。
- `platform/internal/release` 状态机：Pending/Applying/Ready/Failed/Drifted，readback mismatch 重试上限、claim 过期恢复、Failed 重试、Drifted reconcile；合法/非法转移单测覆盖。
- 验证：`0003_releases` 已应用到专用平台库，双库 verify 通过；平台 go test -race 全绿。etcd/APISIX 适配器与 Controller 在 PR-B。

### 阶段 4：适配器与 Controller（PR-B）

- `platform/internal/release` 新增双适配器接口（编译期分离：EtcdAdapter 只管 RPC 注册格式，ApisixAdapter 只管 HTTP 路由格式）与 Desired/Observed 契约；所有写为幂等 upsert，注册中心不可达只重试不删节点。
- Controller：`ClaimNext`（FOR UPDATE SKIP LOCKED 原子认领）、`RunOnce`（执行→回读→转移）、`ExpireStale`（租约过期回 Pending）、`DriftCheck`（只检测不回改）、`CreateOperation`（幂等键去重）。
- 验证：真实平台库随机 schema 集成测试覆盖 Ready 路径与回读落库、幂等键重复提交不重复执行、mismatch 重试到上限 Failed（含租约过期重认领）、执行失败 Fatal、disable 路由、外部改动检测为 Drifted；平台 go test -race 全绿。真实 Compose 三场景验收在 PR-C。

### 阶段 4：真实适配器与 Compose 验收（PR-C）

- 生产适配器：`EtcdDiscovery`（key 为 `<service>/<node>` 的幂等 upsert + 陈旧键清理）与 `ApisixAdmin`（先写上游再写路由；禁用走空 plugins 不动上游）；Admin API 线格式有 httptest 单测。
- 专用验收栈 `platform/deploy/release/`（项目 `mozi-v2-release-acc`）：发现 etcd 与 APISIX Admin 发布在回环，whoami 双实例作上游；凭证为一次性本地验收值。`make v2-acc-up/down`（本机需 `COMPOSE=docker-compose`）。
- `TestReleaseAcceptance`（`MOZI_RELEASE_ACC=1`）三场景全过：①扩缩容 1→2→1，网关观测到两个实例后收敛回单实例；②停止发现 etcd 后网关凭缓存配置继续服务、DriftCheck 不误报漂移，恢复后回读 Ready；③Controller 认领后"崩溃"（租约老化），新实例 ExpireStale 回收并执行到 Ready，网关切换到目标实例。
- 环境注意：APISIX 上游用容器 IP（compose 服务名不走嵌入 DNS 解析链）；whoami 身份依赖 compose `hostname`。验收栈目前保持运行，复查用例可重跑。

## 阶段 4 收口（2026-09-15）

退出条件全部达成，对真实平台代码与专用验收栈（非 PoC fixture）：

| 退出条件 | 证据 |
|---|---|
| 扩缩容 | `TestReleaseAcceptance` 场景 1：上游 1→2→1，网关观测双实例后收敛 |
| 断线恢复 | 场景 2：停发现 etcd，网关凭缓存配置继续服务、无误报漂移、恢复后 Ready |
| Controller 重启可恢复 | 场景 3：租约过期回收 + 新实例执行到 Ready，网关切换目标实例 |

交付链：设计契约与状态机/迁移（PR #18）→ 适配器接口与 Controller（#19）→ 生产适配器与 Compose 验收（#20）。

已知边界：验收栈为独立 Compose 项目 `mozi-v2-release-acc`（回环端口、一次性本地凭证），不用于生产；APISIX 上游注册容器 IP；漂移目前只在主动 DriftCheck 时检测，周期巡检与操作审计检索在阶段 7；发布 API（控制台触发操作）尚未暴露，当前由测试直接驱动 Controller。

### 架构调整：开发平台 web 改为单 SPA（2026-09）

- 用户确认开发平台 web **不使用微应用**：console 与设计器等各工作区同包路由，`platform/web` 移除 micro-app 依赖、独立设计器构建与 `/lab.html` 实验入口；architecture.md 边界已同步修订。
- 设计器（ModelsWorkspace）作为页面组件直接渲染，控制台继续只传项目作用域、路径受限的请求函数，不传原始 token；切换项目重挂载工作区。
- 阶段 0 的 micro-app 集成验证与生命周期回归保留为历史证据，不再代表控制台架构。
- 验证：TypeScript/Vite 单 bundle 构建、九项 Chrome 浏览器用例（登录、项目切换、冲突保留、历史弹窗、设计器页内挂载与登出卸载、移动布局）全部通过。
### 阶段 5：JobIR 与设计库任务定义（PR-A/B）

- 设计契约见 [jobs.md](jobs.md)：execution_id 每次触发独立、同次重试共享（attempt 递增）、超时≠终止、长任务心跳；Dkron 只调度，业务重试全在平台；禁用任务不允许 run 触发。
- 根模块 `mozi.JobIR` 与 `mozi/job` 校验（cron 五段/@every、executor、retry、heartbeat），7 个单测。
- 设计库 `0004_jobs` 新增 design_jobs / design_job_history；`design/jobs` 集合复用 designCollections 鉴权与版本协议，接入成本仅注册一个集合。
- 验证：任务文档的同名隔离、跨项目六类接口 404、viewer 只读、428/409、并发恰一次成功、非法 executor 400、历史失败回滚与删除保留；`0004_jobs` 已应用，双库 verify 通过；平台 go test -race 全绿。执行协议与 Dkron 适配在 PR-C。

### 阶段 6：业务任务协议与 Dkron 适配（PR-C）

- 平台库 `0004_job_executions`：execution_id 全局唯一、running 部分索引、attempt ≥ 1。
- `platform/internal/jobs`：Store（Trigger 独立 ID、Retry 共享 ID 递增 attempt、Complete 幂等拒绝重复完成、Heartbeat、SweepLost 失联清扫、List）；Dispatcher（协议头 X-Mozi-Execution-Id/Attempt/Job/Trigger，超时标记失败不终止业务）；DkronClient（retries 恒为 0，禁用 JobIR 同步为禁用 Dkron 任务，删除容忍 404）。
- 验证：真实平台库随机 schema——两次触发独立 ID、重复完成拒绝、运行中禁止重试、预算耗尽拒绝、心跳失联转 lost 后可重试、列表倒序；httptest 验证 Dkron 负载与调度协议头；慢执行器超时落 failed。平台 go test -race 全绿。Compose 四场景验收在 PR-D。

### 阶段 5：任务端点与 Dkron 验收（PR-D）

- 控制面新增任务端点：Dkron 回调 fire（共享密钥；禁用任务 409）、手动 fire（成员，viewer 403，禁用任务可 ad-hoc）、心跳（204/409）、执行列表。触发 202 异步派发。
- 实测修正：Dkron 4.1.3 的 POST /v1/jobs 只建不改（SyncJob 改为删后建）、任务名拒绝斜杠/点号/大写（`模块-小写名`）、**不投递自定义 executor header**（回调密钥改走 fire_key 查询参数，仅限内网；header 传输在网关阶段重议）。
- `TestJobAcceptance` 四场景对真实 Dkron 全过：①@every 2s 定时触发落 scheduled execution 且成功；②手动触发 202 → succeeded；③超时 attempt 1 失败、Retry 共享 execution_id 于 attempt 2 成功，执行器收到 attempt 头；④心跳 204 两次、停报后 SweepLost 转 lost、lost 后心跳 409。容器经 lima 网关 192.168.5.2 访问宿主机 API。
- 验证：平台 go test -race 全绿。自动重试调度（按 backoff 自动 Retry）与 Dkron 同步生命周期（JobIR 变更自动同步）在后续控制器工作中完善。

## 阶段 5 收口（2026-09-15）

退出条件全部达成，对真实 Dkron 4.1.3 与真实双库：

| 退出条件 | 证据 |
|---|---|
| 可追踪触发/重试 | 定时/手动/retry 三种 trigger 落库可查；execution_id 每次触发独立、同次重试共享（attempt 递增），执行器收到正确协议头 |
| 业务幂等 | Complete 按 (execution_id, attempt) 幂等拒绝重复完成；CreateOperation 幂等键去重；超时标记失败不终止业务 |
| 长任务状态正确 | 心跳 204 → 停报 → SweepLost 转 lost → lost 后心跳 409；lost 可重试 |

交付链：JobIR 设计与校验（PR #22）→ 设计库任务定义 API（#23）→ 执行协议与 Dkron 适配（#25）→ 任务端点与四场景验收（#26）。

实测修正记录（对真实 Dkron 4.1.3）：POST /v1/jobs 只建不改（SyncJob 删后建）；任务名拒绝斜杠/点号/大写（模块-小写名）；不投递自定义 executor header（回调密钥走 fire_key 查询参数）。

已知边界：按 backoff 的自动重试调度与 JobIR 变更的 Dkron 生命周期同步未自动化（当前由端点与测试驱动）；fire_key 经 URL 传输仅限内网，网关阶段重议；任务执行不进入业务请求必经链路的网关接入在阶段 6/7 完成。

### 阶段 6：Release 数据模型与追溯 API（PR-A）

- 平台库 `0005_releases`：releases（design_versions JSONB 快照、code_ref）+ environment_releases（晋级/回退追加式历史，superseded 语义预留）。
- `release.Catalog`：Snapshot 读取三集合当前版本 token；Create 固化快照；List 倒序；Provenance 三方关联。
- 端点：POST/GET releases、GET provenance；viewer 可读不可建，创建写审计。
- 验证：真实双库——快照冻结（创建后改模型，首个 Release 快照不变，第二个捕获新版本）、列表倒序、provenance 空历史、viewer 403、跨项目 404、缺 code_ref 400；`0005_releases` 已应用，双库 verify 通过；平台 go test -race 全绿。晋级/回退编排在 PR-B。
- CI：v2-platform workflow 修复 web job（builder-react 依赖需 npm ci）。

### 阶段 6：晋级与回退编排（PR-B）

- `release.Promoter`：Promote/Rollback 编排——protected 强制 confirm（409）、缺失目标 404/409、成功后旧 ready 转 superseded；任务同步按快照 token 从 design_job_history 加载 JobIR 并 SyncJob 到 Dkron。
- 端点：POST promote / rollback；viewer 403。
- 验证：真实双库 + httptest Dkron——promote 后 Dkron 收到 content-digestjob、protected 无 confirm 409/有 confirm 200、r2 晋级后 r1 superseded、rollback 回到 r1 且重同步、单一 ready 环境 rollback 409、viewer 403、缺失 Release 404。平台 go test -race 全绿。Compose 晋级/部分失败/回退验收在 PR-C。

### 阶段 6：晋级 Compose 验收（PR-C）

- `TestPromotionAcceptance` 对真实 Dkron 全过：①dev 晋级后任务经 Dkron 定时触发并 succeeded；②production 无 confirm 409、有 confirm 200；③停 Dkron 晋级落 failed 中间态，恢复后重试晋级 ready 且 r1 superseded；④rollback 回到 r1，Dkron 配置恢复为快照版本（@every 2s）。
- 实测修正：Dkron 4.1.3 cron 为 6 段（秒在前），适配器把产品级 5 段 cron 前置 "0"；Dkron 重启后 raft 先读后可写，就绪探针改为写探测；晋级步骤失败返回 200 + failed 记录与错误详情（ErrStepFailed），区别于 4xx/5xx。
- 验证：平台 go test -race 全绿。deploy/route 步骤的端到端（阶段 4 适配器接入 Promoter）与迁移审查步骤在后续完善；阶段 6 退出条件的可恢复与可回退已对任务链路验证。
