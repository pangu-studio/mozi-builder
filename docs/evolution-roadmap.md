# Mozi 开发平台演进路线图

> 基于当前 v1 架构分析，按优先级排序的未来演进方向。

---

## 一、统一错误码体系（高优先级 · 基础设施）

### 现状

当前系统完全没有结构化错误类型——所有错误都是字符串消息：

```go
// 当前：所有错误都用 gin.H{"error": err.Error()} 返回
c.JSON(http.StatusInternalServerError, gin.H{"error": "model not found"})
```

`APIIntentConfig.ErrorCases` 已经是 `[]string`（自由文本），为 AI agent 提供错误场景描述，但缺乏机器可读的结构化错误码。整个系统中没有 `ErrorCode` 类型、没有错误码枚举、没有客户端错误码表。

### 设计方案

#### 1. 在 ModelIR 中新增错误码定义

```yaml
# models/content/deck.yaml 中新增
api_intent:
  error_codes:
    - code: "DECK_NOT_FOUND"
      http_status: 404
      category: "resource"        # resource | validation | permission | business | system
      severity: "error"           # error | warn
      message: "牌组不存在或已被删除"
      consumer_facing: true       # 是否可暴露给客户端
      retryable: false
      i18n_key: "error.deck.not_found"
```

对应的 Go IR 类型：

```go
// mozi/types.go 新增
type ErrorCodeIR struct {
    Code           string `yaml:"code" json:"code"`
    HTTPStatus     int    `yaml:"http_status" json:"http_status"`
    Category       string `yaml:"category" json:"category"`
    Severity       string `yaml:"severity" json:"severity"`
    Message        string `yaml:"message" json:"message"`
    ConsumerFacing bool   `yaml:"consumer_facing" json:"consumer_facing"`
    Retryable      bool   `yaml:"retryable" json:"retryable"`
    I18nKey        string `yaml:"i18n_key,omitempty" json:"i18n_key,omitempty"`
}
```

#### 2. 开发平台作为错误码分发中心

```
                    ┌─────────────────────────┐
                    │   Mozi Dev Platform      │
                    │   (设计数据库)             │
                    │                          │
                    │  GET /error-codes        │
                    │  GET /error-codes/:model │
                    │  GET /error-codes/export │
                    └─────┬──────┬──────┬──────┘
                          │      │      │
                    ┌─────┘      │      └─────┐
                    ▼            ▼            ▼
              Go 错误码包   TS 错误码包   错误码文档
              (服务端)      (Web/小程序)   (团队 wiki)
```

#### 3. 代码生成产出

- **Go**：生成 `internal/errors/codes.go` — 类型安全的错误构造函数（`ErrDeckNotFound()`）
- **TypeScript**：生成 `src/api/errorCodes.ts` — 枚举 + 错误消息映射 + 类型守卫
- **文档**：自动生成错误码 Markdown 表格（包含 code、HTTP 状态码、分类、可重试性）
- **i18n**：生成 `locales/zh-CN/errors.json` 和 `locales/en/errors.json`

#### 4. 与变更计划的集成

变更计划中的 `api-contract` 任务区域扩展为包含错误码变更：
- 新增错误码 → 生成对应的错误构造器和客户端处理逻辑
- 修改错误码 → 标记为 breaking change 警告
- 删除错误码 → 标记为废弃，保留常量但标记 `@deprecated`

#### 5. 设计字典扩展

新增 `error_categories` 设计字典：
```
resource | validation | permission | business | system | rate_limit | auth
```

---

## 二、模型可视化增强（高优先级 · 用户体验）

### 现状

当前可视化手段有限：
| 视图 | 方式 | 局限 |
|------|------|------|
| ER 图 | Mermaid DSL → SVG | 仅展示实体和关系，无字段详情、无语义标注 |
| 差异视图 | 文本列表（按类型着色） | 无影响面可视化，无依赖关系展示 |
| 模型概览 | 模块表格 + 模型表格 | 纯列表，无卡片/缩略图预览 |
| YAML 预览 | 原始文本 | 无可视化辅助 |

### 设计方案

#### 1. 模型详情卡片视图（Model Card View）

替代当前的纯表格列表，提供图文并茂的模型卡片：

```
┌──────────────────────────────────────────────┐
│  📦 Deck  牌组                    v20260628  │
│  ─────────────────────────────────────────  │
│  牌组是卡片的容器，用户可创建多个牌组来组织卡片      │
│                                              │
│  📊 8 字段  🔗 3 关系  🏷️ 内容模块             │
│                                              │
│  字段预览:                                    │
│  ┌──────┬──────┬──────┬──────┬──────┐        │
│  │ id   │ name │ desc │ ...  │ ...  │        │
│  │ 🔑   │ ✏️   │ ✏️   │      │      │        │
│  └──────┴──────┴──────┴──────┴──────┘        │
│                                              │
│  关系图:                                      │
│  Deck ──has_many──▶ Card                     │
│  Deck ──belongs_to──▶ User                   │
│  Deck ──belongs_to──▶ Group                  │
│                                              │
│  状态: ⚠️ 待同步   [查看详情] [AI 变更计划]      │
└──────────────────────────────────────────────┘
```

#### 2. 业务语义可视化（Semantic Map）

从 `SemanticConfig` 生成可视化视图：

```
┌─────────────────────────────────────────────────┐
│  业务语义地图 — Deck                              │
│                                                 │
│  🎯 目的: 卡牌组是卡片的容器...                    │
│                                                 │
│  👥 受众: [桌面端用户] [小程序用户]                 │
│                                                 │
│  💎 用户价值: 帮助用户按主题或分类组织卡片           │
│                                                 │
│  📋 业务规则:                                     │
│  ┌─────────────────────────────────────────┐    │
│  │ 1. 软删除 → deleted_at 标记              │    │
│  │ 2. 牌组必须属于某个用户                    │    │
│  │ 3. 删除级联 → 卡片 + 复习状态              │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  🔄 生命周期:                                    │
│  创建 ──→ 编辑 ──→ 移动分组 ──→ 删除(软删除)      │
│                                                 │
│  🔐 权限: [仅操作自己的牌组] [仅移动到自己的分组]    │
└─────────────────────────────────────────────────┘
```

#### 3. 变更影响面图（Change Impact Graph）

`DiffViewer` 当前的文本差异视图升级为交互式影响面图：

```
                   ┌──────────────────┐
                   │  Model: Deck     │
                   │  +field: color   │ ← 新增字段
                   └───┬────┬────┬────┘
                       │    │    │
          ┌────────────┘    │    └────────────┐
          ▼                 ▼                 ▼
   ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
   │ ent/schema/  │ │ internal/    │ │ admin/src/   │
   │ deck.go      │ │ handler/     │ │ pages/deck/  │
   │ +FieldColor() │ │ deck.go      │ │ DeckList.tsx │
   └──────────────┘ │ +bind color  │ │ +color列     │
                    └──────────────┘ └──────────────┘
          ┌────────────┐
          ▼            ▼
   ┌──────────┐ ┌──────────┐
   │ DB 迁移   │ │ API 文档  │
   │ ALTER    │ │ OpenAPI   │
   └──────────┘ └──────────┘
```

#### 4. API 端点拓扑图（API Topology Map）

从 `APIIntentConfig` 和 OpenAPI 资产生成端点关系图：

```
                    ┌──────────────────┐
                    │  GET /api/decks  │ ← list (public)
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │  POST /api/decks │ ← create (auth)
                    └────────┬─────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
     ┌────────▼─────┐ ┌─────▼──────┐ ┌─────▼──────────┐
     │ GET /decks/  │ │ PUT /decks │ │ DELETE /decks/  │
     │ :id          │ │ /:id       │ │ :id             │
     └──────────────┘ └────────────┘ └─────────────────┘
     
     消费者: [miniapp] [desktop] [admin]
     认证: Bearer Token
     错误码: DECK_NOT_FOUND | DECK_ACCESS_DENIED
```

#### 5. 跨表面 UI 预览（Surface Preview）

从 `UIIntentConfig.SurfacesConfig` 生成各端的 UI 预览对比：

```
┌──────────────────────────────────────────────────┐
│  UI 表面预览 — Deck 列表页                         │
│                                                  │
│  ┌─────────┐  ┌─────────┐  ┌──────────┐         │
│  │  Admin  │  │ Desktop │  │ MiniApp  │         │
│  │  管理后台 │  │ 桌面端   │  │  小程序   │         │
│  ├─────────┤  ├─────────┤  ├──────────┤         │
│  │ 表格视图 │  │ 卡片网格 │  │ 列表+搜索 │         │
│  │ 搜索栏  │  │ 拖拽排序 │  │ 下拉刷新  │         │
│  │ 分页器  │  │ 右键菜单 │  │ 长按操作  │         │
│  │ 批量操作 │  │ 侧边栏   │  │ 底部弹窗  │         │
│  └─────────┘  └─────────┘  └──────────┘         │
└──────────────────────────────────────────────────┘
```

---

## 三、数据库迁移生成（高优先级 · 工程效率）

### 现状

当前 `differ` 检测到 `table` 名称变更时会标记为"迁移警告"，但不会生成实际的 SQL 迁移文件。

### 设计方案

从模型差异自动生成数据库迁移文件：

```go
// mozi/migration/ 新包
type Migration struct {
    Version     string   // 时间戳版本号
    Description string   // 变更描述
    Up          string   // 升级 SQL
    Down        string   // 回滚 SQL
    Model       string   // 关联的模型
    ModelVersion string  // 关联的模型版本
}
```

生成策略：
- **新增字段** → `ALTER TABLE ... ADD COLUMN ...`（含默认值处理）
- **删除字段** → 标记为危险操作，生成带备注的 `DROP COLUMN`
- **修改字段类型** → 警告不兼容变更，生成 `ALTER COLUMN ... TYPE ... USING ...`
- **新增关系** → 外键约束或连接表（多对多）
- **索引变更** → `CREATE INDEX` / `DROP INDEX`

产出路径：`migrations/<timestamp>_<description>.up.sql` + `.down.sql`

---

## 四、测试合约生成（中优先级 · 质量保障）

### 设计方案

从 `APIIntentConfig` 和模型字段定义自动生成测试合约：

```yaml
# API intent 扩展
api_intent:
  test_contracts:
    - name: "create_deck_success"
      scenario: "正常创建牌组"
      request: { name: "测试牌组", description: "用于测试" }
      expect: { status: 201, body_contains: { name: "测试牌组" } }
    - name: "create_deck_missing_name"
      scenario: "创建牌组缺少必填字段"
      request: { description: "无名称" }
      expect: { status: 422, error_code: "VALIDATION_ERROR" }
    - name: "get_deck_not_found"
      scenario: "查询不存在的牌组"
      request: { id: "non-existent-uuid" }
      expect: { status: 404, error_code: "DECK_NOT_FOUND" }
```

生成：
- **Go**：表驱动测试（`internal/handler/deck_test.go`）
- **TypeScript**：API 集成测试（`admin/src/api/__tests__/deck.test.ts`）
- **Postman/bruno**：HTTP 请求集合

---

## 五、i18n / 国际化支持（中优先级 · 多语言）

### 现状

模型中的 `label`、`description` 目前只有中文。没有 i18n key 体系。

### 设计方案

#### 1. 字段级 i18n key 自动生成

```yaml
fields:
  - name: name
    type: string
    label: 名称
    i18n_key: "field.deck.name"   # 自动推导或手动指定
```

#### 2. i18n 资源文件生成

```
locales/
├── zh-CN/
│   ├── models.json     # {"deck.name": "名称", "deck.description": "描述"}
│   ├── errors.json     # {"error.deck.not_found": "牌组不存在"}
│   └── enums.json      # {"enum.card_state.new": "新卡"}
├── en/
│   ├── models.json
│   ├── errors.json
│   └── enums.json
```

#### 3. 翻译工作流

- 设计字典中的 `label` 支持多语言
- `mozi export --i18n` 导出待翻译的 key 清单
- `mozi import --i18n` 导入翻译结果
- 变更计划中包含 i18n 翻译任务

---

## 六、可观测性集成（中优先级 · 运维）

### 设计方案

从 `APIIntentConfig` 生成 OpenTelemetry 探针：

```yaml
api_intent:
  observability:
    metrics:
      - name: "deck_create_duration"
        type: "histogram"
        description: "牌组创建耗时"
      - name: "deck_create_total"
        type: "counter"
        description: "牌组创建总数"
    traces:
      - span: "deck.create"
        attributes: ["user_id", "deck_name_length"]
    alerts:
      - name: "high_error_rate"
        condition: "error_rate > 5% for 5m"
        severity: "warning"
```

生成：
- Go：`internal/observability/deck_metrics.go`
- Grafana dashboard JSON

---

## 七、权限 / RBAC 代码生成（中优先级 · 安全）

### 现状

`SemanticConfig.Permissions` 是自由文本数组（如 `"用户只能操作自己的牌组"`），仅供 AI agent 参考。

### 设计方案

将权限声明结构化：

```yaml
semantics:
  permissions:
    - resource: "deck"
      action: "read"
      scope: "own"           # own | all | group
      description: "用户只能查看自己的牌组"
    - resource: "deck"
      action: "write"
      scope: "own"
```

生成：
- 权限常量枚举
- 中间件/装饰器（Go middleware、React hook）
- 权限检查骨架代码

---

## 八、设计 Lint 系统（中优先级 · 最佳实践）

### 设计方案

超越当前的 `mozi validate`（仅做结构校验），增加设计层面的 lint 规则：

| 规则 | 级别 | 说明 |
|------|------|------|
| `missing-label` | error | 字段缺少 label |
| `missing-description` | warn | 模型缺少 description |
| `no-primary-key` | error | 模型缺少主键定义 |
| `orphan-relation` | error | 关系目标模型不存在 |
| `no-semantics` | warn | 模型缺少语义配置 |
| `no-api-intent` | warn | 模型缺少 API 意图定义 |
| `reserved-field-name` | error | 字段名与保留字冲突 |
| `missing-soft-delete` | warn | 建议添加 deleted_at 字段 |
| `table-name-mismatch` | warn | Table 名与模型名不一致 |
| `missing-timestamps` | info | 建议添加 created_at/updated_at |

CLI：`mozi lint --model content/Deck --ruleset strict`

---

## 九、GraphQL 支持（低优先级 · 生态扩展）

### 设计方案

从模型 IR 生成 GraphQL schema：

```graphql
type Deck {
  id: ID!
  name: String!
  description: String
  cards: [Card!]!
  user: User!
  group: Group
  createdAt: DateTime!
  updatedAt: DateTime!
}

type Query {
  deck(id: ID!): Deck
  decks(search: String, page: Int, pageSize: Int): DeckConnection!
}

type Mutation {
  createDeck(input: CreateDeckInput!): Deck!
  updateDeck(id: ID!, input: UpdateDeckInput!): Deck!
  deleteDeck(id: ID!): Boolean!
}
```

---

## 十、客户端 SDK 生成（低优先级 · 开发者体验）

### 设计方案

从 API 意图和 OpenAPI 资产，为各消费者生成类型安全的 SDK：

```
sdk/
├── go/
│   └── memflow/client/deck.go     # Go 客户端
├── typescript/
│   └── packages/api-client/src/deck.ts  # TS 客户端
└── miniapp/
    └── utils/api/deck.js           # 小程序适配
```

生成的 SDK 包含：
- 请求/响应类型定义
- 错误类型及错误码匹配
- 重试策略（根据 `APIIntentConfig.Retryable`）
- 幂等性处理（根据 `APIIntentConfig.Idempotency`）

---

## 十一、多服务架构支持（低优先级 · 架构演进）

### 现状

当前假设所有模型属于同一个 monorepo 后端服务。

### 设计方案

允许模型标记其所属的服务，支持跨服务关系引用：

```yaml
# models/content/deck.yaml
service: memflow-cloud    # 模型所属的服务
api_intent:
  base_path: /api/content
  consumers: [miniapp, desktop]

# 跨服务关系
relations:
  - name: user
    type: belongs_to
    target: user/User
    cross_service: true    # 标记为跨服务引用
    service: memflow-auth  # 目标服务
```

---

## 十二、AI Coding 增强（持续演进）

### 变更计划质量提升

1. **上下文增强**：变更计划附加上下游代码片段，减少 AI agent 的"猜测"
2. **变更验证器**：AI 应用变更后，自动运行 `mozi validate` + 类型检查 + 测试
3. **变更回滚**：AI 变更失败时，自动回滚到变更前状态
4. **变更预览**：在实际应用前，生成"预期 diff"供人工审查

### 模型创建辅助

1. **自然语言→模型 IR**：描述业务需求，AI 生成初始模型定义
2. **模型优化建议**：AI 审查现有模型，建议字段拆分/关系优化
3. **相似模型检测**：检测并提示可复用的模型模式

---

## 演进优先级路线图

```
Phase 1 (近期) — 基础设施
├── 统一错误码体系
├── 模型可视化增强（卡片视图 + 影响面图）
└── 数据库迁移生成

Phase 2 (中期) — 质量与效率
├── 测试合约生成
├── 设计 Lint 系统
├── i18n 国际化支持
└── 可观测性集成

Phase 3 (远期) — 生态扩展
├── 权限/RBAC 代码生成
├── GraphQL 支持
├── 客户端 SDK 生成
└── 多服务架构支持

持续演进
└── AI Coding 增强（变更计划质量、模型创建辅助）
```
