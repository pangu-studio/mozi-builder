# 阶段 4 设计契约：发布操作、双适配器与漂移检测

状态：设计稿。对应 [architecture.md](architecture.md) 阶段 4：etcd 与 APISIX 适配、发布操作、漂移。退出条件：扩缩容、断线恢复与 Controller 重启可恢复——对**真实平台代码**验证，不再用阶段 0 的 PoC fixture 充数。

## 定位与边界

- 平台不进入业务请求与已发布任务的必经链路（架构红线）：APISIX 路由与 etcd 注册一旦写入，平台宕机不影响已发布服务。
- 单个资源只有一个配置管理者；平台之外的直接修改只能被检测为漂移，不被自动回改。
- 数据库迁移回滚独立处理，不在发布状态机内。
- 阶段 4 不接真实业务服务：验证对象是示例服务（阶段 3 的 gen/example 思路）与隔离 Compose 项目 `mozi-v2-poc`。

## 发布状态机

平台库 `release_operations` 持久化目标配置与操作。每次回读验证写 `release_readbacks` 快照（审计线索，append-only 与其他历史表一致）。

```text
Pending ──claim──▶ Applying ──readback match──▶ Ready
  ▲                  │   ▲                         │
  │                  │   └──readback mismatch──────┤ (reconcile)
  │                  │      (attempts < max)       ▼
  │                  │                          Drifted
  │                  ├──readback mismatch (attempts ≥ max)──▶ Failed
  │                  └──fatal error────────────────────────▶ Failed
  └──────retry────── Failed
Applying ──claim expired (controller crash/timeout)──▶ Pending
```

- **幂等**：操作携带 `idempotency_key`（唯一约束），重复提交同一键返回已有操作而不重复执行；执行ID与业务完成分离（调度成功 ≠ 业务完成，同 Dkron 约定）。
- **唯一管理者**：`(project_id, resource)` 上的部分唯一索引（`Pending`/`Applying` 状态）保证同一资源同时只有一个活动操作。
- **Controller 恢复**：崩溃或租约过期的 `Applying` 操作回到 `Pending` 可被重新认领——恢复语义是**继续执行到回读验证**，不是盲目重放已完成动作；所有适配器写操作本身幂等（upsert 语义）。
- **漂移**：`Ready` 资源的周期回读与期望不一致 → `Drifted`，人审后 reconcile；不自动覆盖平台外修改。

## 双适配器（不可混用）

| 适配器 | 职责 | 格式红线 |
|---|---|---|
| etcd | RPC 服务注册/注销（go-zero zRPC 发现） | go-zero/etcd 键值格式，仅供 RPC |
| APISIX | HTTP 路由 upsert/禁用、上游实例增删 | Admin API upstream 格式，仅供 HTTP |

- 两种注册格式不可混用（架构红线）；适配器接口在编译期分离，不共享配置结构。
- APISIX 零实例时禁用路由以空 plugins 满足 schema，不引用未创建的上游（PoC 教训）。
- 注册中心断线时已下发的配置继续使用——这不是漂移；漂移检测在连接恢复后进行。

## 发布顺序

兼容迁移 → 部署 → 就绪验证（回读）→ 路由切换 → 任务与前端启用。阶段 4 实现部署与路由切换段；任务启用属阶段 5。

## 平台库迁移 `0003_releases.sql`

- `release_operations(id, project_id, resource, kind, desired JSONB, idempotency_key UNIQUE, state, attempts, last_error, actor_id, request_id, created_at, updated_at)`；`kind`：`deploy | scale | route | disable`。
- `release_readbacks(id, operation_id, observed JSONB, match BOOLEAN, created_at)`，触发器禁止 UPDATE/DELETE（复用审计模式）。

## PR 切分

| PR | 内容 | 验收 |
|---|---|---|
| A | 本设计文档 + `0003_releases` 迁移 + 状态机包与单测 | 迁移双库 verify；转移合法性/非法性单测 |
| B | etcd/APISIX 适配器 + Controller 幂等执行与租约恢复 | 适配器契约测试；崩溃恢复单测（注入时钟与失败） |
| C | 真实 Compose 三场景验收：扩缩容、断线恢复、Controller 重启 | 证据记入 progress.md，对真实平台代码 |

## 与 PoC 的差异（不得直接搬运）

阶段 0 的 HTTP 任务状态是内存 fixture、任务 header 用固定测试 ID、APISIX key 是一次性实验凭证——这些**不得**进入平台运行代码（progress.md 已声明）。PR-B 起，Controller 操作与凭证来自平台库与显式配置。
