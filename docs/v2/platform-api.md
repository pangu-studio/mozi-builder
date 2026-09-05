# 平台基础 API（阶段 1）

平台使用 `platform/cmd/platform-api`，go-zero 独立模块。启动时只读核验双库迁移；版本缺失、漂移、未知版本或数据库不可用均拒绝启动。

## 初始化和启动

从仓库根目录执行：

```sh
make v2-migrate
make v2-migrate-verify
# 先在环境中安全设置 MOZI_USER_PASSWORD（12–72 字节），不要把密码放在命令参数中。
cd platform
go run ./cmd/platform-user -env-file ../.env -email user@example.com -name 管理员
go run ./cmd/platform-api -env-file ../.env
```

默认监听 `127.0.0.1:15180`。部署时显式传 `-host`、`-port`；外部入口必须由 HTTPS 网关终止 TLS。没有默认用户、公开注册或硬编码密码。`platform-user` 用于创建平台用户，重复邮箱会失败；创建项目的用户自动成为项目 owner。

## 契约

所有接口前缀 `/api/v2`，JSON 使用 snake_case。登录成功得到 `access_token`，其余请求携带 `Authorization: Bearer <token>`。

| 方法/路径 | 输入或输出 | 权限 |
|---|---|---|
| POST /login | email, password → access_token, token_type, expires_in | 公开，来源地址限流 |
| POST /logout | 撤销当前会话，204 | 登录 |
| GET /me | id, email, display_name | 登录 |
| GET /projects | 当前用户所属项目数组（id, slug, name, role） | 登录 |
| POST /projects | slug, name → id | 登录，自动成为 owner |
| GET /projects/:id/environments | 环境数组（id, slug, name, kind） | 项目成员 |
| POST /projects/:id/environments | slug, name, kind → id | owner / maintainer |
| GET /projects/:id/members | 成员数组（user_id, role） | 项目成员 |
| POST /projects/:id/members | user_id, role → id；新增或修改已有非 owner 成员 | owner |

角色为 owner / maintainer / developer / viewer。成员写入接口只接受后三种角色，禁止降级 owner。项目外的用户访问项目子资源统一返回 404；项目内权限不足返回 403。环境 kind 为 development / staging / production，production 自动标记 protected；后续部署操作必须继续检查该标记。

slug 为 2–63 个字符，以小写字母开头，后续为小写字母、数字或连字符。重复 slug 或不存在的成员用户返回 409；非法输入返回 400；响应不泄漏底层 SQL 或连接信息。列表按 ID 排序，当前单次最多 500 条，完整分页后续实现。

## 会话、审计和边界

会话为 256-bit 随机值，数据库仅保存 SHA-256 摘要，固定有效期 12 小时。每次认证检查数据库会话和用户禁用状态，退出立即撤销当前 token；当前不提供刷新、找回密码、SSO 或 UI 登录页。

密码采用 bcrypt。登录请求按 TCP 来源地址在数据库中限制每分钟 20 次，不信任客户端提供的 X-Forwarded-For。经过反向代理的用户目前共享代理地址的限额，接入 APISIX 时需配置可信代理识别。go-zero 请求日志关闭，避免凭证进入请求日志。过期 sessions 和超过保留期的 login_limits 需运维定期清理，当前没有后台清理任务。

项目/环境/成员成功变更、登录成功、退出和已认证用户的项目权限拒绝写入审计；变更与审计同事务，审计失败会回滚变更。当前未记录匿名登录失败和输入校验失败。审计表触发器禁止 UPDATE/DELETE；具有 DDL 权限的数据库所有者仍能修改触发器，运行账号与迁移账号的进一步分权在部署阶段完成。

此阶段项目只登记在平台库；设计库的 design_projects 和模型作用域在阶段 2 接入，不做双库事务。当前没有项目删除、owner 转移、环境部署或审计检索接口。

## 设计服务契约（阶段 3）

服务契约（ServiceIR）接口前缀 `/api/v2/projects/:project/design/services`，与阶段 2 模型接口逐项对齐：列表上限 500、POST 同名 409、PUT/DELETE 缺版本 428、版本不匹配 409、跨项目或非成员 404、viewer 只读、删除保留历史（历史最近 100 条）。请求体为 `{ "document": <ServiceIR> }`，PUT/DELETE 另带读取时的 `version`。

文档经根模块 `mozi/service` 校验：拒绝未知属性、非法标识、无效字段类型、消息/路由引用缺失、proto 编号越界或落入 19000–19999、复用 reserved 编号或字段名。字段编号规则与破坏性变更分类见 [service-ir.md](service-ir.md)。模型与服务不互相级联删除；服务引用不存在模型由后续 lint 报告，不在写入时阻断。

## 验证

```sh
cd platform
go test -race ./...
go vet ./...
# 绝对路径指向专用 v2 数据库配置。测试创建随机 schema 并在结束时清理。
MOZI_INTEGRATION_ENV=/absolute/path/to/mozi-builder/.env go test -race ./internal/control
```

集成测试覆盖迁移幂等、路由鉴权、跨项目隔离、viewer 写拒绝、owner 保护、冲突响应、会话撤销/过期/禁用、登录限流、审计不可变与审计失败回滚。没有配置 MOZI_INTEGRATION_ENV 时会明确跳过 PostgreSQL 集成用例。
