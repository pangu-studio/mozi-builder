# 阶段 2：项目模型与版本隔离

## 数据边界

v2 模型保存在 `mozi_v2_design.design_models`，唯一键为 `(project_id, module, name)`。模型的完整 ModelIR 使用 JSONB 保存，设计库仍为事实来源。模型写入同时生成 `design_model_history` 快照，保存操作者、操作类型和时间；触发器禁止修改或删除历史。不会迁移或修改旧设计库。

项目作用域由平台 API 校验会话及项目成员关系后取得。首次写模型时，设计库用平台查询到的项目 ID、slug、名称登记 `design_projects`；请求体不能指定其他项目。平台库的成员行锁持有至设计操作完成，以免撤权与写入竞争；两个库没有分布式事务，模型和历史的一致性完全在设计库内保证。

角色：owner / maintainer / developer 可建模，viewer 只读。跨项目或非成员请求返回 404。读取、列表、历史、创建、更新、删除均校验项目成员关系，所有 SQL 都带 project_id。

## API

前缀 `/api/v2/projects/:project/design`，使用平台 Bearer 会话。

| 方法 | 路径 | 契约 |
|---|---|---|
| GET | /models | 项目模型列表，最多 500 条 |
| POST | /models | `{ "document": <ModelIR> }`；同名返回 409 |
| GET | /models/:module/:name | 返回 `{module,name,version,document,updated_at}` |
| PUT | /models/:module/:name | `{ "version": "读取时的版本", "document": <完整 ModelIR> }` |
| DELETE | /models/:module/:name | `{ "version": "读取时的版本" }` |
| GET | /models/:module/:name/history | 最近 100 个历史快照，包含被删除模型 |

模型 JSON 的名称键为 `model`，模块为 `module`。后端复用固定仓库版本的 ModelIR 类型及 parser.Validate；拒绝未知属性、无主键、非法字段类型以及不合法的模型/模块/表标识。更新时身份不可变，修改 module 或 model 必须与路径一致。

版本为每次提交重新生成的不透明 token。更新/删除缺版本返回 428，版本不匹配返回 409，数据库执行条件更新，过期编辑不会覆盖新内容。删除只标记模型，保留定义和历史；同名暂不可复建，也不自动恢复已删除模型。

模型变更审计位于设计库历史表，不跨库写平台 audit_events。成员拒绝目前不额外写入设计变更历史。高级关系目标解析、发布契约、Diff/ChangePlan 在后续建模生成阶段完善；当前不做跨模型级联删除。

## 控制台

选择项目后进入“模型设计”。micro-app 子应用复用现有 FieldTable / FieldEditor；模型基本属性和字段可视化编辑，semantics、admin、ui_intent、api_intent、relations 等扩展定义采用完整 JSON 编辑，保存后可查看历史快照。

基座只提供绑定当前项目的请求函数，不向子应用传原始 token。函数限制设计接口路径，子应用卸载时清空通信数据并中止请求。切换项目销毁旧实例与编辑状态；编辑未保存时提示确认。冲突保留本地草稿，用户主动重新读取最新版本后再编辑。v1 组件库、Gin 接口和 `/lab.html` 演示入口保持兼容。

## 运行和验证

先 `make v2-migrate` 应用设计库 `0002_models`，再重启平台 API。数据库迁移只作用于专用 v2 库。

```sh
cd platform
MOZI_INTEGRATION_ENV=/absolute/path/to/mozi-builder/.env go test -race ./...
go vet ./...
cd web
npm run build
PLAYWRIGHT_CHANNEL=chrome npm run test:e2e
```

数据库集成测试使用双库的随机临时 schema 并清理，覆盖同名隔离、跨项目六类接口拒绝、viewer 拒绝写、版本必填、并发竞争、历史失败回滚和删除历史保留。浏览器测试以模拟 API 验证编辑、冲突草稿、历史和项目切换。根模块 `go test ./...` 用于 v1 回归。
