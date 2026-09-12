# Mozi v2 架构与实施基线

状态：2026-09-05 用户已确认实施。部署目标为 Docker Compose，暂不考虑 Kubernetes。

## 边界

- 保留根 Go module、Gin 嵌入接口和 builder-react 发布入口。v2 平台在 platform/ 独立 Go module 中开发，避免框架依赖升级影响 v1。
- 平台先采用 go-zero 模块化 API + 独立 Controller。业务模型归属服务，模块不等于部署单元；禁止跨服务数据库外键、JOIN 与级联删除。
- console-shell、designer-app、operations-app 通过 micro-app 集成。先验证现有设计器，不重写模型 UI。
- APISIX 负责外部入口；go-zero RPC 使用 etcd 注册发现；HTTP 注册通过适配器转成 APISIX 上游。两种注册格式不可混用。
- Dkron 独立持久化调度状态；业务任务自行处理逻辑执行 ID、幂等与长任务结果。调度成功不等于业务完成。
- 平台不进入业务请求与已发布任务的必经链路。

## 数据与配置

- mozi_v2_design：独立设计库，模型、契约、设计版本。MOZI_DB 必须显式指定。
- mozi_v2_platform：独立平台库，项目、环境、成员、凭证引用、发布、操作、审计。MOZI_PLATFORM_DB 必须显式指定。
- 两库已创建且最初为空；不得自动回退到 memflow_design、mozi 或其他现有库，不导入现有业务数据。
- 跨库用稳定字符串 ID 关联，不建立跨库事务。迁移各自有版本、校验和与锁；启动 API 不自动执行迁移。
- 项目/环境权限必须由服务端校验。设计版本与代码版本通过发布记录关联，生成产物不能成为第二个任意可编辑的设计源。
- 控制面持久化目标配置与操作，Controller 幂等执行、回读验证。状态为 Pending / Applying / Ready / Failed / Drifted。
- 单个资源只有一个配置管理者。发布顺序为兼容迁移、部署、就绪验证、路由切换、任务与前端启用；数据库回滚独立处理。

## 阶段及验收

| 阶段 | 范围 | 退出条件 |
|---|---|---|
| 0 ✅ | 架构固化、版本锁定、独立 Compose PoC | 微前端、HTTP→RPC、实例→网关、任务执行四条链路有实测证据 |
| 1 ✅ | go-zero 平台骨架、双库迁移、认证、项目/环境、权限、审计 | 空库启动路径可复现；跨项目拒绝；无旧库回退 |
| 2 ✅ | 设计器平台化、多项目设计隔离、乐观版本 | 同名模型跨项目隔离；旧入口回归通过 |
| 3 ✅ | ServiceIR、gozero-ent、稳定 proto 编号、ChangePlan | 增量更新保留手写代码；HTTP/RPC 示例可运行（2026-09-07 达成，证据见 progress.md） |
| 4 | etcd 与 APISIX 适配、发布操作、漂移 | 扩缩容、断线恢复与 Controller 重启可恢复 |
| 5 | JobIR、Dkron、业务任务协议 | 可追踪触发/重试，业务幂等与长任务状态正确 |
| 6 | Release、CI、Compose 发布、环境晋级 | 产物可追溯；部分失败可恢复，配置/应用可回退 |
| 7 | HA、审计检索、观测、凭证轮换、恢复演练 | 按确认后的容量及可用性目标验收；无 Kubernetes 范围 |

## 交付

v2 为集成分支；feat/v2-* 分支提交 PR 到 v2。阶段状态与证据记录在 progress.md；未验证项必须明确列出。PoC 中的简化实现不能直接被标记为生产能力。
