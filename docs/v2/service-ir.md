# 阶段 3 设计契约：ServiceIR、稳定 proto 编号与生成链路

状态：设计稿，待评审。对应 [architecture.md](architecture.md) 阶段 3：ServiceIR、gozero-ent、稳定 proto 编号、ChangePlan。退出条件：增量更新保留手写代码；HTTP/RPC 示例可运行。

## 定位与边界

- ServiceIR 是**服务契约 IR**，与 ModelIR（数据模型）并列。ModelIR 回答"数据长什么样"，ServiceIR 回答"对外暴露什么操作、消息字段编号是多少"。ServiceIR 消费 ModelIR，不复制其字段定义。
- 类型定义放**根模块 `mozi`**（新增 `mozi/service.go` 与 `mozi/service/` 校验/编号包），与 ModelIR 同一事实来源。platform 模块经伪版本引用根模块，与阶段 2 消费 ModelIR/parser 的方式一致；v1 的 CLI、devplatform、builder-react 行为不变（纯新增，不改既有类型）。
- proto 编号是契约的一部分，**显式存储**，绝不从字段位置推导。
- 不做：跨服务数据库外键/JOIN/级联删除（架构红线）、Kubernetes、生产发布与路由切换（阶段 4/6）、业务任务的持久化执行协议（阶段 5）。

## ServiceIR 结构

```yaml
# 单个服务一份文档，键与 ModelIR 风格一致
schema_version: 1
module: content            # 归属模块，与 ModelIR 的 module 对齐
service: ContentService    # PascalCase
label: 内容服务
description: 牌组与卡片的读写契约
domain: content            # 业务域，用于错误码与审计归属

messages:                  # 服务内消息定义，编号在此显式管理
  - message: DeckSummary
    fields:
      - { name: id,    type: string, number: 1 }
      - { name: title, type: string, number: 2 }
      - { name: due,   type: time,   number: 3 }
    reserved_numbers: [4]  # 曾删除的 review_count
    reserved_names: [review_count]

http:                      # 对外 HTTP 契约（经网关）
  - name: ListDecks
    method: GET
    path: /api/content/decks
    response: DeckSummaryList
    auth: jwt              # jwt | admin | public
    idempotency: none
    error_codes: [DECK_NOT_FOUND]
rpc:                       # 内部 RPC 契约（etcd 发现）
  - name: GetDeck
    request: GetDeckRequest
    response: DeckSummary
    idempotency: read_safe
```

- 字段类型复用 ModelIR 的 8 种 `FieldType`，另加 `message:<Name>`（引用同服务消息）、`model:<module/Model>`（从模型投影生成消息字段子集）与 `repeated` 修饰。映射到 proto 标量的规则集中在 `mozi/service`，生成器不做二次判断。
- 错误码复用根模块已有 `ErrorCodeIR`（v1 已定义），ServiceIR 只引用 code，不重定义。
- 一个服务文档唯一键为 `(project_id, module, service)`，与模型的 `(project_id, module, name)` 模式对齐。

## proto 稳定编号规则

这是本阶段最重要的兼容性约束：

1. **显式存储**：每个消息字段的 `number` 写在 ServiceIR 里，随文档入设计库与历史快照。生成 .proto 时原样输出，不重排。
2. **新增字段**：取该消息 `max(number, reserved_numbers) + 1`，由校验器建议、写入文档后生效。
3. **删除字段**：编号进 `reserved_numbers`、字段名进 `reserved_names`，**永不复用**；生成的 .proto 保留 `reserved` 声明。未走删除流程直接从文档抹掉字段视为校验失败。
4. **重命名**：使用 `renamed_from`（与 FieldIR 相同的机制），编号不变，不算破坏性变更。
5. **编号变更**：已有字段改编号一律 breaking，differ 必须报出，校验器拒绝安全路径应用。
6. **类型变更**：仅 wire 兼容的标量转换可判 safe（如 `int32↔int64` 不在我们的 8 类型内，实际规则为：同一 FieldType 不变；`int→float`、`string→text` 判 conditional；其余判 breaking）。
7. **保留区间**：proto 规定的 19000–19999 禁止使用，校验器强制。

differ 需要扩展 ServiceIR diff：字段新增/删除/编号变更/类型变更按上面规则给出 safe / conditional / breaking 结论，供 ChangePlan 消费。

## 设计库存储与平台 API

- 新迁移 `0003_services.sql`：`design_services` + `design_service_history`，完全复用 `0002_models` 的模式——JSONB 存完整 ServiceIR、复合项目唯一键、每次写入生成不可变历史快照、触发器禁止改历史、乐观版本 token 条件写入。不建跨表外键。
- API 前缀 `/api/v2/projects/:project/design/services`，契约与阶段 2 模型接口逐项对齐：列表上限 500、同名 409、缺版本 428、版本不匹配 409、跨项目/非成员 404、viewer 只读、删除保留历史。鉴权复用现有项目成员校验与行锁模式。
- 模型与服务同属设计域，但**不互相级联删除**；引用悬空（服务引用了不存在的模型）由校验/lint 报告，不在写入时硬阻断。

## gozero-ent 生成

- 复用根模块 generator 的模板引擎（`[[ ]]` 分隔符、embed fs）与 **marker 系统**（`// mozi:section` 等），这是"增量更新保留手写代码"退出条件的实现手段；禁止整文件覆盖手写区。
- 产物清单（示例项目验收用）：
  - `.proto`：由 ServiceIR 消息与 RPC 渲染，含 `reserved` 声明；
  - go-zero `.api` 文件：由 HTTP 契约渲染；
  - ent schema 骨架：由引用的 ModelIR 渲染（gozero-ent 含义：go-zero 服务 + ent 存储的模板组合）；
  - handler/logic 骨架：marker 分区，业务实现留手写区。
- 生成物是**应用侧代码**，不回写设计库；设计库仍是唯一事实来源（架构文档红线）。

## ChangePlan 接入 v2

- v1 的 ChangePlan 装配逻辑在 `devplatform/service.go`（`buildChangeIntent`/`buildChangeTasks`/`buildChangeChecks`/`buildChangePrompt`），目前是纯函数但与 v1 `db.Store` 同包。抽到根模块独立包（如 `mozi/changeplan`），v1 devplatform 改为薄适配层，保证 v1 行为回归通过。
- v2 数据源换成 `design_models` / `design_services` 的当前文档 + 历史快照，`differ.Compare` 直接复用；ServiceIR diff 按上节规则扩展。
- 输出 `ChangePlanResult` JSON，经平台 API 暴露；控制台 ChangePlan 界面属于后续 PR，不在本阶段验收条件内。

## 实施顺序（PR 切分）

platform 经伪版本引用根模块，根模块改动必须先合入并推送，platform 才能 `go get` 消费，因此严格按依赖顺序切分：

| PR | 内容 | 验收 |
|---|---|---|
| A | 根模块：ServiceIR 类型、`mozi/service` 校验与编号规则、ServiceIR differ 扩展，纯单测 | `go test ./...`（含 v1 回归） |
| B | platform：`0003_services.sql` + 服务 CRUD/历史 API（go get PR-A 的伪版本） | 双库临时 schema 竞态测试，模式同阶段 2 |
| C | 根模块：proto / .api / ent schema / handler 模板与渲染，golden 文件测试 | 渲染产物 diff 可审查，marker 替换单测 |
| D | 根模块 changeplan 抽取 + platform 接入 + **示例服务端到端** | HTTP 请求与 RPC 调用在示例项目跑通，作为阶段退出证据 |

## 验证基线

- 根模块 `go test ./...`（v1 回归，每个 PR 必跑）；platform `go test -race ./...` 与 `go vet ./...`。
- 平台集成测试沿用阶段 2 模式：双库随机临时 schema，覆盖跨项目拒绝、角色边界、乐观版本并发、历史回滚。
- 编号规则负例：复用 reserved 编号、改已有编号、19000 区间，三者都必须被校验拒绝并有测试。
- 阶段退出时，示例 HTTP/RPC 服务的运行证据记入 [progress.md](progress.md)，未验证项明确列出。
