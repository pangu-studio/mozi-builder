# 阶段 5 设计契约：JobIR、业务任务协议与 Dkron 适配

状态：设计稿。对应 [architecture.md](architecture.md) 阶段 5：JobIR、Dkron、业务任务协议。退出条件：可追踪触发/重试，业务幂等与长任务状态正确。

## 定位与边界

- Dkron 独立持久化**调度**状态；业务任务自行处理逻辑执行 ID、幂等与长任务结果。**调度成功不等于业务完成**（架构红线，PoC 已实测重试与超时语义）。
- JobIR 是任务定义 IR，存设计库（项目作用域、乐观版本、不可变历史），与 ModelIR/ServiceIR 同模式；执行记录存平台库，不跨库事务。
- 阶段 5 不进入业务请求必经链路；任务执行器是业务侧 HTTP 端点，平台只调度与观测。

## 业务任务协议（核心）

PoC 的固定 header 只是测试 fixture。正式协议：

| 概念 | 规则 |
|---|---|
| `execution_id` | **每次触发生成独立 ID**（定时触发、手动触发、重试后的再次调度各自独立）；由平台生成，ULID/随机文本 |
| `attempt` | **同次重试共享 execution_id**，attempt 从 1 递增；重试由平台按任务重试策略发起 |
| 幂等 | 执行器必须按 `(execution_id)` 幂等：同一 execution_id 的重复投递（重试、网络重发、Dkron 重复触发）不得重复生效 |
| 超时 | attempt 超过任务 timeout 标记失败并触发重试策略，**不代表业务终止**；业务侧仍在跑的由其自行收尾 |
| 长任务 | `long_running` 任务要求执行器按 `heartbeat_interval` 回报心跳；心跳中断超过宽限期判定 attempt 失联失败 |
| 完成 | 执行器同步返回 2xx 表示 attempt 完成；长任务可异步回报完成状态 |

请求头：`X-Mozi-Execution-Id`、`X-Mozi-Attempt`、`X-Mozi-Job`、`X-Mozi-Trigger`（scheduled | manual | retry）。

## JobIR

```yaml
schema_version: 1
module: content
job: DeckDigestJob
label: 牌组摘要任务
description: 每日为每个牌组生成复习摘要
schedule: "0 3 * * *"        # cron 五段式，或 @every 30s
executor:
  kind: http
  method: POST
  path: /jobs/deck-digest    # 业务侧执行端点，经网关
timeout_seconds: 300
retry:
  max_attempts: 3
  backoff_seconds: 60        # 固定间隔；指数退避后续扩展
long_running: false
heartbeat_interval_seconds: 30  # long_running 时必填
enabled: true
```

校验：名称 PascalCase、module snake_case；cron 五段或 `@every <dur>`；timeout ≥ 1；max_attempts ≥ 1；long_running 必须给 heartbeat_interval；executor.path 以 `/` 开头。暂停的 `enabled=false` 任务**不参与调度也不允许 Dkron run 触发**（Dkron 4.1.3 禁用任务不能 run——PoC 教训）；平台的"立即执行"创建一次性手动 execution，不依赖 Dkron run 接口。

## 存储

- 设计库 `0004_jobs.sql`：`design_jobs` / `design_job_history`，完全复用 models/services 模式（JSONB、复合项目键、乐观版本、append-only 历史）。
- 平台库（PR-B/C 迁移）：`job_executions(id, project_id, job, execution_id UNIQUE, trigger, attempt, state, started_at, finished_at, heartbeat_at, error)`；同一 `(job, execution_id)` 一行，attempt 递增更新。状态：`running | succeeded | failed | lost`。

## Dkron 适配

- 适配器把 enabled 的 JobIR 同步为 Dkron job：调度来自 Dkron，执行参数（execution_id 等）由平台在收到 Dkron 触发回调时生成——**固定 header 是 PoC fixture，正式实现必须由平台在触发时注入执行 ID**。
- Dkron 重试机制不参与业务重试：业务重试由平台按 retry 策略驱动（同 execution_id 递增 attempt）；Dkron 侧 retries 置 0，避免双重重试语义（PoC 的 Dkron 重试验证仅证明调度层可用）。
- 暂停（`enabled=false`）时从 Dkron 删除或禁用对应 job；恢复时重新同步。

## PR 切分

| PR | 内容 | 验收 |
|---|---|---|
| A | 本设计文档 + 根模块 JobIR 类型与校验（mozi/job） | 根模块单测 + v1 回归 |
| B | 设计库 `0004_jobs` + 任务定义 CRUD/历史 API（复用 designCollections） | 双库集成测试，模式同 services |
| C | 平台 `job_executions` + 执行协议（触发、重试、心跳、完成）+ Dkron 适配器 | 协议单测：独立 execution_id、同次重试共享、幂等拒绝 |
| D | Compose 验收：定时触发、手动触发、超时重试、长任务心跳四场景 | 证据记入 progress.md |

## 与 PoC 的差异

PoC 的 Dkron 实验（重试完成、1s 超时失败、@every 3s 定时）证明调度层可用，但其固定执行 ID、内存任务状态不得进入平台代码（progress.md 已声明）。手动任务在 PoC 中保持启用+远期调度是变通；正式语义以本协议为准。
