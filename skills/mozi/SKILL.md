---
slug: mozi
name: mozi
displayName: Mozi 模型驱动开发
version: 0.2.1
description: 使用 mozi CLI 进行模型驱动开发。当需要创建或修改业务模型、校验或 lint ModelIR、检查差异与 AI 变更计划、管理错误码或设计字典、导入导出 YAML 快照，以及生成受控的数据库迁移、Bruno 合约、权限骨架、i18n 目录或 OpenAPI TypeScript SDK 时使用。
---

# Mozi 模型驱动开发

通过 CLI 完成建模、校验、变更分析、代码修改和契约产物生成。不要要求用户切换到浏览器完成 Agent 可通过 CLI 完成的操作。

## 事实来源

- 将 PostgreSQL 设计数据库视为模型定义的主存储。
- 将 `models/` YAML 视为 Git 快照和交换格式，不要默认把它当作日常编辑源。
- 使用 `mozi model get/create/update` 修改模型；不要直接写设计数据库，也不要用 HTTP 请求替代 CLI。
- 将 ModelIR 用于领域语义、数据结构和产品意图，不要只描述数据库表。
- 将 ProjectIR 错误码注册表作为错误码事实来源；模型 API 意图只引用已注册错误码。
- 将生成的 `docs/swagger.json` 作为 HTTP 契约事实来源。Bruno 合约和 TypeScript SDK 必须从 OpenAPI 生成。
- 将 API Workbench 中的端点归属等信息视为展示性维护数据，不要用它修改 HTTP 方法、路径或 Schema。
- 将迁移、合约、权限、翻译目录和 SDK 视为待审查产物；“成功生成”不代表“获准执行”。
- 使用宿主应用的 `make dev` 启动 Builder UI；Mozi 不提供独立 HTTP 服务命令。

## 建模必答项

每个模型必须覆盖以下五类问题。缺少信息时先询问，不要猜测后直接保存。

| 关注点 | ModelIR 字段 | 必须确认 |
|---|---|---|
| 领域语义 | `semantics` | 目的、受众、用户价值、业务规则、权限、生命周期 |
| 数据结构 | `fields`、`relations`、`table` | 字段、约束、关系、表名、重命名意图 |
| 管理后台 | `admin` | 列表字段、搜索字段、排序、分页 |
| 产品 UI | `ui_intent` | 用户任务、统一术语、空状态、各端差异 |
| API 契约 | `api_intent` | 暴露范围、消费者、认证、操作、错误码、测试合约、版本策略 |

### 关系规则

每个 `relations[]` 必须包含业务谓词 `label`。不要把 `name`、`back_ref` 或 ORM 类型当作业务谓词。

常用关系表达：

| 业务含义 | 示例 label |
|---|---|
| 容器与内容 | `包含`、`归集` |
| 所有权 | `拥有`、`归属` |
| 行为产生结果 | `创建`、`产生`、`发布` |
| 状态表达 | `表示`、`跟踪` |
| 事件记录 | `记录`、`触发` |
| 分类关联 | `关联`、`隶属于` |

从当前模型视角使用主动谓词，并检查反向关系能否讲述一致的业务故事。例如：Deck `包含` Card，Card `归属` Deck。

### 字段重命名规则

字段重命名时设置 `fields[].renamed_from`。不要把重命名表达成未标注的“删除旧字段＋新增字段”；后者会被判定为破坏性变更。即使显式标注重命名，也必须人工审查条件型迁移。

### 结构化权限规则

优先使用 `semantics.permission_rules`，保留 `semantics.permissions` 仅用于旧模型兼容和自然语言补充。

```yaml
permission_rules:
  - effect: allow
    principal: user
    resource: deck
    action: update
    scope: own
    owner_field: user_id
```

- 使用 deny-first、fail-closed 语义。
- `own` 必须提供 `owner_field`；`tenant` 必须提供 `tenant_field`。
- 只把前端权限判断用于界面展示；真正授权必须在服务端执行。
- 对生成器不支持的复杂 `condition`，保留在应用策略代码中，不要自动放宽。

### 错误码与测试合约

先注册错误码，再从 `api_intent.error_codes` 或 `test_contracts.expect.error_code` 引用。

```bash
mozi error-code upsert DECK_NOT_FOUND \
  --domain content --status 404 --category resource \
  --message '牌组不存在' --consumer-facing
```

测试合约必须引用稳定的 OpenAPI `operation_id`：

```yaml
test_contracts:
  - name: get_deck_not_found
    operation_id: getDeck
    request:
      path: { id: missing-id }
    expect:
      status: 404
      error_code: DECK_NOT_FOUND
```

## 安装 Mozi CLI

先检查本机是否已有可执行文件：

```bash
command -v mozi && mozi --version
```

### 版本一致性检查（必须执行）

每次开始使用本 Skill 时，读取 frontmatter 中的 `version`，并与 `mozi --version` 输出的版本号比较。

- 版本一致：继续执行任务。
- CLI 未安装：按下方平台说明引导安装。
- CLI 版本低于或不同于 Skill 版本：先明确提醒用户升级 Mozi CLI，并提供 GitHub Releases 安装方式。
- 未完成升级前，不要调用当前 CLI 可能尚未支持的新命令或新字段；可以继续执行与旧版本明确兼容的只读检查。
- 升级后再次运行 `mozi --version`，确认与 Skill 版本一致，再继续写操作。

提醒示例：

> 当前 Mozi Skill 版本为 `0.2.1`，本机 CLI 版本为 `<实际版本>`，两者不一致。建议先从 GitHub Releases 升级 CLI；升级并确认版本一致后再继续模型写入或产物生成。

未安装时，从 [GitHub Releases](https://github.com/pangu-studio/mozi-builder/releases) 下载当前平台产物。不要从不明镜像下载，也不要跳过校验和检查。

### macOS / Linux

发布产物支持 `darwin`、`linux` 的 `amd64`、`arm64`。使用以下命令自动识别平台并安装到用户目录：

```bash
set -euo pipefail
case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) echo "不支持的系统: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "不支持的架构: $(uname -m)" >&2; exit 1 ;;
esac
asset="mozi_${os}_${arch}.tar.gz"
base="https://github.com/pangu-studio/mozi-builder/releases/latest/download"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -fL "$base/$asset" -o "$tmp/$asset"
curl -fL "$base/checksums.txt" -o "$tmp/checksums.txt"
expected="$(awk -v file="$asset" '$2 == file {print $1}' "$tmp/checksums.txt")"
actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
test -n "$expected" && test "$actual" = "$expected"
tar -xzf "$tmp/$asset" -C "$tmp"
mkdir -p "$HOME/.local/bin"
install -m 0755 "$tmp/mozi_${os}_${arch}/mozi" "$HOME/.local/bin/mozi"
"$HOME/.local/bin/mozi" --version
```

确保 `$HOME/.local/bin` 已加入 `PATH`。没有 `curl` 或无法访问 GitHub 时，停止并让用户手动提供可信的 Release 产物；不要自行改用第三方下载站。

### Windows PowerShell

```powershell
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$asset = "mozi_windows_${arch}.zip"
$base = 'https://github.com/pangu-studio/mozi-builder/releases/latest/download'
$tmp = Join-Path $env:TEMP "mozi-install-$PID"
New-Item -ItemType Directory -Force $tmp | Out-Null
Invoke-WebRequest "$base/$asset" -OutFile (Join-Path $tmp $asset)
Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')
$expected = ((Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { $_ -match "\s$([regex]::Escape($asset))$" }) -split '\s+')[0]
$actual = (Get-FileHash (Join-Path $tmp $asset) -Algorithm SHA256).Hash.ToLower()
if (-not $expected -or $actual -ne $expected.ToLower()) { throw 'Mozi CLI 校验和不匹配' }
Expand-Archive (Join-Path $tmp $asset) -DestinationPath $tmp -Force
$bin = Join-Path $HOME 'bin'
New-Item -ItemType Directory -Force $bin | Out-Null
Copy-Item (Join-Path $tmp "mozi_windows_${arch}\mozi.exe") (Join-Path $bin 'mozi.exe') -Force
& (Join-Path $bin 'mozi.exe') --version
```

将 `$HOME\bin` 加入用户 `PATH`。安装后始终运行 `mozi --version`，确认 CLI 可执行且版本符合预期。

## 环境

```bash
export MOZI_DB='postgres://localhost:5432/memflow_design?sslmode=disable'
export MOZI_PROJECT_ROOT='/absolute/path/to/business-project'
```

未设置 `MOZI_PROJECT_ROOT` 时，CLI 会向上查找 `go.mod`。数据库连接被拒绝时，明确说明本地 PostgreSQL/设计数据库不可用，不要伪造结果。

沙箱无法读取用户 Go 缓存时使用：

```bash
GOCACHE=/private/tmp/memflow-go-build-cache go test ./...
```

## 命令速查

### 初始化与快照

```bash
mozi new myapp --module github.com/example/myapp --desktop --miniapp
mozi init
mozi import --dir models/
mozi import --file models/content/deck.yaml
mozi export --dir models/
mozi export --module content
```

### 校验、差异与历史

```bash
mozi validate
mozi validate --module content
mozi lint --strict
mozi lint --json
mozi diff --model content/Deck
mozi history --model content/Deck
```

### 模型 CRUD

```bash
mozi model get --model content/Deck --json
mozi model create --json '<完整 ModelIR>'
mozi model update --model content/Deck --json '<完整 ModelIR>'
```

`model update` 需要完整 ModelIR。始终先 `get`，修改完整 JSON 后再 `update`；不要提交局部对象，否则遗漏字段会被清空。

### 变更计划与同步

```bash
mozi change-plan --model content/Deck
mozi change-plan --model content/Deck --json
mozi sync --model content/Deck
mozi sync --all
```

### 错误码与设计字典

```bash
mozi error-code list --json
mozi error-code delete DEPRECATED_CODE

mozi dictionary list api_consumers --json
mozi dictionary upsert api_consumers desktop --label '桌面端' --alias tauri --json
mozi dictionary delete api_consumers legacy_consumer --json
```

### Phase 2 契约产物

```bash
mozi artifacts migration --model content/Deck --out migrations
mozi artifacts bruno --model content/Deck --openapi docs/swagger.json --out contracts/bruno
mozi artifacts permissions --model content/Deck --out internal/permissions/generated.go
mozi artifacts i18n --locale zh-CN --out locales/source.json
mozi artifacts i18n-validate --locale en --input locales/en.json
mozi artifacts typescript-sdk --openapi docs/swagger.json --out sdk/typescript/client.ts
```

## 创建模型工作流

1. 按五类关注点了解需求，明确假设。
2. 展示完整 ModelIR，并在用户确认后保存：

```bash
mozi model create --json '<完整 ModelIR>'
```

3. 执行设计校验：

```bash
mozi validate
mozi lint --strict
mozi diff --model <Module/Model>
```

4. 获取 AI 变更计划：

```bash
mozi change-plan --model <Module/Model>
```

5. 按计划制作最小、可审查的普通代码补丁；保留现有业务逻辑和用户改动。
6. 修改 Swagger 注解后重新生成 OpenAPI：

```bash
swag init -g cmd/server/main.go -o docs/
```

7. 根据模型内容生成必要的 Phase 2 产物，不要无条件全部生成。
8. 执行生成、类型检查和测试：

```bash
make generate
cd admin && npx tsc --noEmit
cd .. && GOCACHE=/private/tmp/memflow-go-build-cache go test ./...
mozi export --module <Module>
```

9. 审查代码和快照差异后同步：

```bash
mozi sync --model <Module/Model>
```

## 修改模型工作流

1. 获取完整模型：

```bash
mozi model get --model <Module/Model> --json > current.json
```

2. 修改完整 JSON，并明确 rename、权限、错误码和契约变化。
3. 更新设计数据库：

```bash
mozi model update --model <Module/Model> --json "$(cat current.json)"
```

4. 依次执行 `validate`、`lint --strict`、`diff`、`change-plan`。
5. 先审查 breaking/conditional/dangerous 项，再修改代码。
6. 重新生成 OpenAPI 和必要的契约产物。
7. 完成类型检查、测试和 YAML 导出后再执行 `sync`。

## 批量模型变更

1. 一次性创建或更新所有相关模型。
2. 执行 `mozi validate && mozi lint --strict`。
3. 获取每个变化模型的 change plan，跳过状态为 `applied` 的模型。
4. 合并成一个覆盖完整关系影响面的代码补丁。
5. 重新生成 OpenAPI 和相关契约产物。
6. 执行全量验证并审查 YAML 快照。
7. 使用 `mozi sync --all` 同步已确认完成的模型。

## 契约产物规则

### 数据库迁移

- 仅允许 `mozi artifacts migration` 自动生成全部为 `safe` 的迁移。
- 命令拒绝 conditional/dangerous 时立即停止；展示迁移建议并请求人工决策。
- 不要削弱门禁，不要自动执行 Change Plan 中展示的 SQL。
- 删除字段、类型收窄、复杂类型转换和不可逆操作必须人工设计数据迁移。
- 审查 `.up.sql` 与 `.down.sql`，尤其关注锁表、默认值、历史数据和回滚数据损失。

### Bruno 合约

- 仅在 `api_intent.test_contracts` 非空时生成。
- 先更新并审查 OpenAPI，确保 `operation_id` 稳定。
- 把 Bruno 当作黑盒 HTTP 契约，不用它替代领域单元测试。

### 权限骨架

- 仅在 `semantics.permission_rules` 非空时生成。
- 生成常量和 `Authorizer` 接口后，在服务端显式接入 enforcement point。
- 为 allow、deny、own、tenant 和默认拒绝编写测试。

### i18n

- 导出 source catalog 后，将翻译存为 key→string JSON 对象。
- 缺失 key 或占位符不一致时视为失败。
- 将 stale key 作为清理候选，不要未经确认直接删除线上翻译。
- 不要自动伪造未经审核的翻译。

### TypeScript SDK

- 只从已审查的 OpenAPI 生成。
- 生成后检查 diff，并在实际消费者中运行 TypeScript 类型检查。
- 不要从自由文本 API Intent 猜测请求或响应结构。

## 常见影响路径

- 后端：`ent/schema/`、`internal/model/`、`internal/handler/`、`internal/service/`
- 前端：`admin/src/pages/`、`admin/src/api/`、`admin/src/stores/`
- OpenAPI：`docs/swagger.json`、`docs/swagger.yaml`
- 迁移：`migrations/*.up.sql`、`migrations/*.down.sql`
- 合约：`contracts/bruno/*.bru`
- 权限：`internal/permissions/generated.go`
- i18n：`locales/source.json`
- SDK：`sdk/typescript/client.ts`

将影响分析中的 `certain`、`inferred`、`suggested` 分开处理。不要把路径推断或 AI 建议描述成确定事实。

## 安全规则

- 不要使用旧的模板覆盖工作流。
- 始终通过 `mozi change-plan` 获取变更契约，不要用 `curl` 或浏览器代替。
- 不要修改与模型变更无关的文件。
- 保留用户未提交改动；失败回滚只能覆盖本次受控修改。
- 修改 ent Schema 后运行 `make generate`。
- 修改 Swagger 注解后运行 `swag init -g cmd/server/main.go -o docs/`。
- 修改前端后运行 TypeScript 类型检查。
- 模型变更完成后导出 YAML 快照并检查 `git diff models/`。
- 只有完成代码、迁移、契约和验证后才能执行 `mozi sync`。
- 如果数据库或外部系统操作需要额外权限，先请求授权，不要绕过审批。

## 最终报告

说明：

- 修改了哪些模型和契约。
- `validate` 与 `lint --strict` 是否通过。
- 是否使用了 Change Plan，是否存在 breaking/conditional/dangerous 项。
- 生成了哪些迁移、Bruno、权限、i18n 或 SDK 产物。
- 执行了哪些类型检查和测试。
- YAML 是否导出、模型是否同步。
- 仍需人工完成或审批的事项。
