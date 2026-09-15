# 阶段 6 设计契约：Release、环境晋级与可回退发布

状态：设计稿。对应 [architecture.md](architecture.md) 阶段 6：Release、CI、Compose 发布、环境晋级。退出条件：产物可追溯；部分失败可恢复，配置/应用可回退。

## 定位与边界

- Release 把三类已有事实**关联**起来：设计版本（design_models/services/jobs 的 version token）、代码版本（git sha / 镜像 digest）、发布操作（阶段 4 状态机）。不引入新的编辑入口——设计库仍是唯一事实来源。
- 发布顺序固定：兼容迁移 → 部署 → 就绪验证 → 路由切换 → 任务与前端启用。**数据库回滚独立处理**（架构红线），不在回退流程内。
- 单个资源只有一个配置管理者（阶段 4 已落地）；晋级与回退都是普通 release_operation 序列，共享租约恢复。
- CI 门禁见 `.github/workflows/v2-platform.yml`（独立 PR），不重复设计。

## Release 数据模型（平台库 `0005_releases.sql`）

```text
releases(
  id TEXT PRIMARY KEY,
  project_id REFERENCES projects(id),
  label TEXT NOT NULL,                    -- 人类可读，如 "2026.09-卡片契约"
  design_versions JSONB NOT NULL,         -- {"models":{"content/Card":"<token>"},"services":{...},"jobs":{...}}
  code_ref TEXT NOT NULL,                 -- git sha 或镜像 digest，可追溯
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ
)
environment_releases(                      -- 每个环境的当前/历史指针
  environment_id REFERENCES environments(id),
  release_id REFERENCES releases(id),
  action TEXT CHECK(action IN ('promote','rollback')),
  state TEXT NOT NULL,                    -- applying | ready | failed | superseded
  created_at TIMESTAMPTZ,
  PRIMARY KEY(environment_id, release_id, action, created_at)
)
```

- `design_versions` 在创建 Release 时快照各集合当前版本 token——发布后设计库继续演进不影响已固化 Release（生成产物不能成为第二设计源）。
- 同一 Release 可晋级到多个环境；`environment_releases` 追加式记录晋级/回退动作及其状态。

## 环境晋级

- `promote(release, environment)` 展开为操作序列：兼容迁移（人工审查产物，非自动执行）→ 部署（阶段 4 deploy/route 操作）→ 就绪验证（回读）→ 任务同步（阶段 5 Dkron SyncJob）→ 前端启用。每步独立 release_operation，失败停留中间态：可重试或整体回退。
- `production` 环境的 `protected` 标记（阶段 1 已有）强制二次确认；服务端强制，前端仅展示。
- staging 与 production 之间没有代码差异：晋级只搬 Release 引用，不重新构建。

## 回退

- `rollback(environment)` = 以该环境**上一个 ready 的 Release** 重新执行 promote 序列，`action='rollback'`。
- 回退不包含数据库回滚；迁移的兼容性是发布顺序的第一位（先兼容迁移）正是为此。
- 配置回退：APISIX/etcd 的期望态来自 Release 固化的 desired 快照，回退即重新发布旧快照。

## 追溯查询

- `GET /api/v2/projects/:id/releases`：项目 Release 列表（含 design_versions 摘要）。
- `GET /api/v2/projects/:id/releases/:rid/provenance`：该 Release 的完整三方关联（设计版本 → 代码引用 → 各环境发布操作与状态）。
- 审计：晋级/回退写入 audit_events（阶段 1 模式，同事务）。

## PR 切分

| PR | 内容 | 验收 |
|---|---|---|
| A | 本设计文档 + `0005_releases` 迁移 + Release 创建/列表/追溯 API | 双库集成；快照固化不被后续设计修改影响 |
| B | promote/rollback 序列编排（复用阶段 4 操作与阶段 5 任务同步） | 中间态失败可重试；protected 强制确认 |
| C | Compose 验收：dev→staging 晋级、部分失败注入与恢复、回退到上一 Release | 证据记入 progress.md |

## 与既有能力的衔接

| 已有 | 本阶段用法 |
|---|---|
| 阶段 4 状态机/适配器 | 部署与路由切换步骤 |
| 阶段 5 DkronClient | 任务同步步骤（SyncJob/DeleteJob） |
| 阶段 2/3/5 设计集合 version token | design_versions 快照来源 |
| 阶段 1 environments.protected / audit | 晋级审批与审计 |
