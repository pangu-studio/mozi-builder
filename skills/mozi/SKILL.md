---
name: mozi
slug: mozi
displayName: Mozi Model-Driven Development
version: 0.1.0
description: Model-driven development using the mozi CLI and embedded builder UI context. Use when Codex/Claude Code needs to create or modify business models/entities, validate model definitions, build AI Coding change plans, import/export YAML snapshots, inspect model diffs/history, manage design dictionaries, understand OpenAPI-derived API assets, or reason about endpoint-to-module associations maintained in the builder UI.
---

# Mozi Model-Driven Development

This skill is the primary automation surface for Codex/Claude Code to interact with mozi. It encapsulates the full model-driven workflow: create/modify models, validate, diff, fetch change-plan, manage design dictionaries, and drive incremental code generation — all via CLI without requiring browser UI interaction. The embedded builder UI includes API Workbench for humans to inspect OpenAPI-derived API assets and curate lightweight metadata such as endpoint-to-module association.

Use this skill inside `memflow-cloud` when work involves mozi models, generated CRUD code, or the dev-platform.

## Source of Truth

- The design database PostgreSQL is the primary store for model definitions.
- `models/` YAML files are Git snapshots and exchange format; do not treat them as the daily editing source unless the user explicitly asks for YAML import/export work.
- Models can be created/modified via mozi CLI or the builder UI — both write to the same design database. The UI is primarily for human viewing, visual modeling, ER diagrams, API asset inspection, API debugging, and lightweight curation.
- ModelIR includes database structure, business semantics, and UI intent. Do not reduce modeling work to table design only.
- CLI is the only automation surface for Codex/Claude Code. Prefer `mozi ...` commands for mozi work; do not automate mozi work through builder UI network calls, direct HTTP calls, or direct database writes.
- The dev-platform no longer uses template overwrite as its main path. Save the model, inspect the diff, fetch the AI change plan, then let Codex/Claude Code create a normal incremental patch in the repository.
- Generated OpenAPI (`docs/swagger.json`) is the source for API contract facts in the API Workbench.
- API Workbench curation, such as endpoint-to-module association, is stored separately from OpenAPI facts. Do not change HTTP method, URL, request schema, or response schema from Workbench state; contract changes must be made in code and reflected in regenerated OpenAPI.
- The builder UI is embedded into the host application. In MemFlow, start it with `make dev`; mozi does not provide a standalone HTTP server command.
- Detailed human-facing usage lives in `docs/dev-platform-guide.md`; read it when the user asks for operational guidance or platform usage docs.

## Modeling Conversation Framework

Model definitions are produced through human-AI dialogue. The AI agent is responsible for guiding the conversation and mapping requirements to ModelIR. This framework ensures consistency across conversations and agents.

### Hard Rule: Cover Five Concern Areas

Every model definition MUST address all five concern areas. If the user hasn't mentioned some, ask.

| # | Concern Area | ModelIR Fields | What to Ask |
|---|-------------|---------------|-------------|
| 1 | **Domain Semantics** | `SemanticConfig` (purpose, audience, user_value, business_rules, permissions, lifecycle) | "What problem does this model solve? Who uses it? What business rules must be enforced? What's its lifecycle?" |
| 2 | **Data Structure** | `Fields`, `Relations`, `Table` | "What fields does it need? What's each field's type? How does it relate to other models? For every relation, what business predicate should appear in the ER diagram? Should the table name differ from the default?" |
| 3 | **Admin Panel Intent** | `AdminConfig` (list_columns, search_fields, default_sort, default_order, page_size) | "Which columns should appear in the admin list? What should be searchable? How should results be sorted?" |
| 4 | **Product UI Intent** | `UIIntentConfig` (product_goal, user_tasks, shared, surfaces_config) | "How will users interact with this across surfaces (admin/desktop/miniapp)? What are the primary user tasks? What does the empty state look like?" |
| 5 | **API Intent** | `APIIntentConfig` (exposure, consumers, auth, operations, error_cases, rate_limit, versioning) | "Who calls this API? What auth is needed? What operations are required? What are the key error cases?" |

### Hard Rule: Every Relation Has A Business Predicate

Every `relations[]` entry MUST include `label`. The label is the business predicate shown on ER diagram edges and in model documentation. It is not the same as:

- `name`: code navigation property, such as `cards`, `deck`, `owner`
- `back_ref`: reverse navigation property, such as `deck`, `review_logs`
- `type`: ORM relation type, such as `has_many` or `belongs_to`

#### Display Precedence

ER diagrams display `label` first. When `label` is missing, a type-based fallback is used:

| Relation Type | Fallback Label |
|--------------|----------------|
| `has_many` | 拥有 |
| `has_one` | 拥有 |
| `belongs_to` | 属于 |
| `many_to_many` | 关联 |

**These fallbacks are only "approximately correct."** They mechanically describe the ORM direction, not the business reality. Real modeling MUST derive the label from business semantics — the fallback exists only so a never-labeled model is at least readable, not so you can skip labeling.

#### Deriving Labels from Business Semantics

The label answers: **"In the domain language, what is the nature of the connection between these two things?"** To find the right label, read the `semantics.business_rules` and `semantics.lifecycle` of the current model. The predicate that best captures the business relationship is the label.

Do NOT mechanically pick the same label for every relation of the same type. Two `belongs_to` relations on the same model may need different labels because they represent different business concepts.

**Predicate categories** (use to brainstorm, not as a fixed menu):

| Category | Labels | When to Use |
|----------|--------|-------------|
| containment | `包含`, `归集`, `划分为` | Parent holds/collects/organizes children |
| authorship/action | `创建`, `提交`, `审核`, `发布`, `产生` | One entity actively generates another |
| state/representation | `表示`, `刻画`, `跟踪` | One entity models or reflects another's state |
| event/logging | `记录`, `触发` | An entity captures an event about another |
| ownership/belonging | `拥有`, `归属`, `隶属于` | Ownership or organizational membership |
| classification | `关联`, `属于`, `归属于` | Loose association or categorization |

#### Canonical Examples (MemFlow)

These are the verified, semantically-correct labels for this project. Use them as reference for how labels connect to business semantics:

```yaml
# Deck — a container of cards, optionally placed in a group
relations:
  - name: user     # "牌组必须属于某个用户" → ownership
    label: 归属
    type: belongs_to
    target: user/User
    back_ref: decks
  - name: cards    # "牌组包含卡片"
    label: 包含
    type: has_many
    target: content/Card
    back_ref: deck
  - name: group    # "卡牌组可关联到分组下"
    label: 归集
    type: belongs_to
    target: content/Group
    back_ref: decks

# Card — the core reviewable content
relations:
  - name: deck         # "卡片必须属于某个牌组"
    label: 归属
    type: belongs_to
    target: content/Deck
    back_ref: cards
  - name: state        # 卡片通过复习状态跟踪记忆进度
    label: 跟踪
    type: has_one
    target: content/CardState
    back_ref: card
  - name: review_logs  # "复习日志在每次评分时创建并关联卡片"
    label: 产生
    type: has_many
    target: content/ReviewLog
    back_ref: card

# CardState — FSRS memory state, computed client-side
relations:
  - name: card     # "CardState 表示 Card 的复习状态"
    label: 表示
    type: belongs_to
    target: content/Card
    back_ref: state
  - name: user     # "复习状态归属用户"
    label: 归属
    type: belongs_to
    target: user/User
    back_ref: states

# User — system user, owns all personal data
relations:
  - name: decks       # 用户创建并拥有牌组
    label: 拥有
    type: has_many
    target: content/Deck
    back_ref: user
  - name: states      # 用户通过复习状态跟踪学习进度
    label: 跟踪
    type: has_many
    target: content/CardState
    back_ref: user
  - name: review_logs # 用户创建复习日志（每次评分产生一条）
    label: 创建
    type: has_many
    target: content/ReviewLog
    back_ref: user

# ReviewLog — immutable record of each review event
relations:
  - name: card     # 日志记录卡片的复习事件
    label: 记录
    type: belongs_to
    target: content/Card
    back_ref: review_logs
  - name: user     # 日志归属用户
    label: 归属
    type: belongs_to
    target: user/User
    back_ref: review_logs

# Group — tree-structured deck organizer
relations:
  - name: user       # 分组归属用户
    label: 归属
    type: belongs_to
    target: user/User
    back_ref: groups
  - name: parent     # "分组可隶属于父分组"
    label: 隶属于
    type: belongs_to
    target: content/Group
    back_ref: children
  - name: children   # 分组包含子分组
    label: 包含
    type: has_many
    target: content/Group
    back_ref: parent
  - name: decks      # 分组归集牌组
    label: 归集
    type: has_many
    target: content/Deck
    back_ref: group
```

#### Labeling Procedure

When creating or modifying a model:

1. **Read the semantics** you've written for the current model — `business_rules` and `lifecycle` contain the domain language.
2. **For each relation**, ask: "What business predicate connects this model to the target?" Write it as a verb in the active voice from the current model's perspective.
3. **Cross-check with the target model's semantics** — the two sides of the relation should tell a consistent story (e.g., Deck `包含` Card ↔ Card `归属` Deck).
4. **Avoid generic repetition** — if every `belongs_to` label is `归属`, you're not thinking about business semantics. Compare CardState `表示` Card vs CardState `归属` User — same type, different predicates, both correct.
5. **State assumptions** — when the correct predicate isn't obvious from the semantics, state your assumption and confirm with the user before saving.

Do not save a model with a relation that only has `name`, `type`, `target`, and `back_ref`.

### Conversation Phases

**Phase 1 — Understand**: Elicit domain semantics first. Clarify purpose, audience, business rules, and lifecycle. Don't jump to fields — semantics shape the data model.

**Phase 2 — Model**: Based on the semantic understanding, propose:
- Fields with types, constraints, and defaults
- Relations to other models, with a required business predicate in `relations[].label`
- Admin list/search/sort configuration
- UI intent (shared terminology, user tasks, surface configurations)
- API intent (exposure, consumers, operations, error cases)

**Phase 3 — Confirm**: Present the complete ModelIR (as JSON or structured summary). Confirm with the user before writing to the design database. Highlight any assumptions made.

### Mapping User Statements to ModelIR

| User Says | Map To |
|-----------|--------|
| "Users need to search by name" | `fields[name].searchable = true`, `admin.search_fields = [name]` |
| "This is for internal use only" | `api_intent.exposure = "internal"` |
| "Only the owner can edit" | `semantics.business_rules`, `api_intent.error_cases` (403) |
| "It needs a due date" | `fields` + `time` type + required |
| "I want to filter by status" | `fields[status].listable = true`, `admin.list_columns` |
| "Support offline in the desktop app" | `ui_intent.surfaces_config.desktop.constraints` |
| "The mini-program should be minimal" | `ui_intent.surfaces_config.miniapp.constraints` |
| "CLI needs machine-readable output for AI agents" | `ui_intent.surfaces_config.cli.constraints` |
| "X contains Y" / "X 包含 Y" | `relations[].label = "包含"` on X's `has_many` to Y |
| "X generates Y" / "X 产生 Y" | `relations[].label = "产生"` on X's `has_many` to Y |
| "X belongs to Y" / "X 归属 Y" | `relations[].label = "归属"` on X's `belongs_to` to Y |
| "X collects Y" / "X 归集 Y" | `relations[].label = "归集"` on X's `has_many` to Y |

### Field Rename Caveat

The differ detects a field rename as "remove old field + add new field". When a rename is needed, explicitly confirm with the user that this is a rename (not a delete+add), and note in the change-plan tasks that data migration may be required. Future versions of the platform may support explicit migration hints (e.g., `migrations.field_renames`).

## Environment

mozi requires two environment variables to locate the design database and the business project:

```bash
# PostgreSQL design database connection string
export MOZI_DB='postgres://localhost:5432/memflow_design?sslmode=disable'

# Business project root (the project whose models you want to manage)
export MOZI_PROJECT_ROOT='/absolute/path/to/memflow-cloud'
```

If `MOZI_PROJECT_ROOT` is not set, mozi will search upward from the current directory for a `go.mod` file.

If database commands fail with connection refused, tell the user the local PostgreSQL/design DB is unavailable and retry after they reconnect it.

When Go test fails only because the sandbox cannot read the user Go build cache, use a workspace-safe cache:

```bash
GOCACHE=/private/tmp/memflow-go-build-cache go test ./...
```

## Command Reference

Run mozi commands from any directory when `MOZI_PROJECT_ROOT` is set. Otherwise run from `memflow-cloud` (or any descendant directory) so mozi can locate the project root by searching for `go.mod`.

```bash
# Start MemFlow server with integrated dev-platform UI/API
make dev

# Scaffold a brand-new mozi-builder-based project
mozi new <name> --module <go-module>
mozi new myapp --module github.com/foo/myapp --desktop --miniapp

# Initialize design DB tables and model directory
mozi init

# Import YAML snapshots into design DB
mozi import --dir models/
mozi import --file models/content/deck.yaml

# Export design DB snapshots to YAML
mozi export --dir models/
mozi export --module content

# Validate models in design DB
mozi validate
mozi validate --module content

# Diff and history
mozi diff --model content/Deck
mozi diff --model content/Deck --from 3 --to 5
mozi history --model content/Deck

# Model CRUD (direct to design DB, no server needed)
mozi model get --model content/Deck
mozi model get --model content/Deck --json
mozi model create --json '{...}'
mozi model update --model content/Deck --json '{...}'

# AI Coding change plan (CLI, no server needed)
mozi change-plan --model content/Deck
mozi change-plan --model content/Deck --json

# Design dictionaries (CLI, no server needed)
mozi dictionary list api_consumers --json
mozi dictionary upsert api_consumers desktop --label '桌面端（Tauri）' --alias 桌面端 --alias tauri --json
mozi dictionary delete api_consumers legacy_consumer --json

# Record model as synced in the manifest
mozi sync --model content/Deck
mozi sync --all
```

Makefile aliases:

```bash
make mozi-init
make mozi-import
make mozi-export
make mozi-validate
make mozi-diff MODEL=content/Deck
make mozi-history MODEL=content/Deck
```

Note: mozi HTTP routes are embedded by the host application. The current UI path is the integrated MemFlow server started with `make dev`.

`mozi new` does NOT require a design database (PostgreSQL) — it only creates files. Run it from anywhere; it bootstraps a new project directory. No `--project-root` / `go.mod` needed.

## Scaffold A New Project

`mozi new` creates a fresh mozi-builder-based monorepo from just a project name and Go module path. The web frontend is `<name>-ui` by default and sits beside the backend (大仓). Tauri desktop (`<name>-desktop`) and WeChat mini-program (`<name>-miniapp`) clients are optional and scaffolded from minimal skeletons derived from the MemFlow reference stacks.

### Quick Start

```bash
# Minimal: backend + web frontend
mozi new myapp --module github.com/example/myapp

# Full: backend + web + desktop + miniapp
mozi new myapp --module github.com/example/myapp --desktop --miniapp

# Customise web frontend dir
mozi new myapp --module github.com/example/myapp --ui-dir myapp-admin
```

### What It Creates

```
<name>/
├── cmd/server/main.go          # Gin + dev-platform + CORS + SPA static + /api/ping
├── internal/middleware/        # cors.go, builder.go (stub — replace with real auth)
├── models/_project.yaml        # Mozi project config
├── docs/dev-platform-guide.md  # Placeholder guide
├── Makefile                    # MOZI ?= mozi, dev/build/generate/ui-* targets
├── go.mod, .env, .env.example, .gitignore, CLAUDE.md, README.md
├── <name>-ui/                  # React + Vite + Ant Design + MoziBuilderProvider
├── <name>-desktop/             # [--desktop] Tauri v2 + React + Tailwind + greet example
└── <name>-miniapp/             # [--miniapp] Taro 4 + React + Sass + WeChat login stub
```

### Post-Scaffold Steps

```bash
cd <name>

# 1. Resolve Go dependencies (set GOPROXY for private module if needed)
GOPROXY=https://goproxy.cn,direct go mod tidy

# 2. Install web frontend deps
cd <name>-ui && npm install && cd ..

# 3. (if --desktop) Install desktop deps
cd <name>-desktop && npm install && cd ..

# 4. (if --miniapp) Install miniapp deps
cd <name>-miniapp && npm install && cd ..

# 5. Initialize design database (PostgreSQL required)
export MOZI_DB=postgres://localhost:5432/<name>_design?sslmode=disable
mozi init

# 6. Start dev servers
<NAME>_DEV_PLATFORM=true make dev          # backend → :8080
cd <name>-ui && npm run dev                # UI → :5173 (proxies /api → :8080)
cd <name>-desktop && npm run tauri dev     # desktop (if enabled)
cd <name>-miniapp && npm run dev:weapp     # miniapp (if enabled)
```

### Design Notes

- **No ent/Swagger on day 1.** These are added by the mozi workflow when the first model is generated (see Create A Model below).
- **Builder auth is a stub** (`internal/middleware/builder.go`). Replace the token-check middleware with real JWT/role-based auth before production.
- **FSRS not scaffolded.** The scheduling engine is client-specific (Rust for Tauri, WASM for miniapp). Templates carry `TODO` comments referencing the architecture in CLAUDE.md.
- **`mozi new` is not a Makefile target** — it bootstraps the project that the Makefile lives in.

## Core Workflows

This skill automates the full change-plan → code generation loop. When the user describes a model change, follow this workflow without requiring them to switch to a browser.

### Create A Model

1. Follow the Modeling Conversation Framework above to elicit all five concern areas. Build the ModelIR JSON incrementally as the conversation progresses.
2. Present the complete ModelIR to the user for confirmation. Once confirmed, save it directly to the design database via CLI (no server needed):

```bash
mozi model create --json '<complete-model-ir>'
```

3. Validate and inspect the diff:

```bash
mozi validate
mozi diff --model <Module/Model>
```

4. Fetch the change-plan via CLI (auto-handled by this skill):

```bash
mozi change-plan --model <Module/Model>
```

5. Apply the returned intent/tasks/contracts as a normal incremental code patch, preserving existing code.
6. Regenerate Swagger/OpenAPI docs so the admin UI API Workbench reflects the updated API contract:

```bash
swag init -g cmd/server/main.go -o docs/
```

7. Verify:

```bash
make generate
cd admin && npx tsc --noEmit
cd .. && GOCACHE=/private/tmp/memflow-go-build-cache go test ./...
mozi export --module <Module>
```

8. Record the generation in the manifest so the change-plan shows "applied":

```bash
mozi sync --model <Module/Model>
```

9. Report model changes, code patch summary, checks, and any remaining manual work.

### Modify A Model

1. Read the current model to understand its full state:

```bash
mozi model get --model <Module/Model> --json > current.json
```

2. Discuss the desired changes with the user. Modify the JSON (keeping all fields — `update` requires a complete payload, not a partial patch). Save the updated model:

```bash
mozi model update --model <Module/Model> --json "$(cat current.json)"
```

3. Validate and inspect the diff:

```bash
mozi validate
mozi diff --model <Module/Model>
```

4. Fetch the AI change plan via CLI and implement the smallest repository patch that satisfies it:

```bash
mozi change-plan --model <Module/Model>
```

5. Regenerate Swagger/OpenAPI docs so the admin UI API Workbench reflects the updated API contract:

```bash
swag init -g cmd/server/main.go -o docs/
```

6. Verify:

```bash
make generate
cd admin && npx tsc --noEmit
cd .. && GOCACHE=/private/tmp/memflow-go-build-cache go test ./...
mozi export --module <Module>
```

7. Record the generation in the manifest:

```bash
mozi sync --model <Module/Model>
```

### Batch Model Changes

When multiple models change (e.g., a new model + relation to existing model), process them together:

1. Create/update all models in the design DB.
2. Validate all models: `mozi validate`
3. Fetch change-plan for each changed model. Skip models whose status is "applied".
4. Generate a unified code patch covering all affected files.
5. Regenerate Swagger/OpenAPI docs: `swag init -g cmd/server/main.go -o docs/`
6. Run full verification.
7. Sync all changed models:

```bash
mozi sync --all
```

### Sync YAML And Database

- YAML to DB:

```bash
mozi import --dir models/
```

- DB to YAML:

```bash
mozi export --dir models/
```

Run `git diff models/` after export and mention changed snapshots.

## Generated Files

Backend per model:

- `ent/schema/{module}_{model}.go`
- `internal/model/{module}/{model_snake}.go`
- `internal/handler/{module}/{model_snake}.go`
- `internal/service/{module}/{model_snake}.go`

Frontend per model:

- `admin/src/pages/{module}/{Model}List.tsx`
- `admin/src/pages/{module}/{Model}Form.tsx`
- `admin/src/api/{module}.ts`
- `admin/src/App.tsx`

`admin/src/api/{module}.ts` and `admin/src/App.tsx` are merged with mozi markers. Do not delete marker blocks unless intentionally removing generated routes/API entries.

## Admin UI Context

The admin UI is part of the main MemFlow server started with `make dev`. It is useful for human review, visual checks, API Workbench inspection/debugging, and lightweight curation such as endpoint-to-module association.

Agents should not automate mozi work through admin UI network calls. Use the mozi CLI for model CRUD, validation, diff, change-plan, sync, import, and export. Treat admin UI requests as frontend implementation details unless the user explicitly asks to debug the admin UI or verify browser behavior.

## Safety Rules

- Do not use template overwrite as the dev-platform workflow.
- Use the AI change plan as the contract, then edit the repository through normal reviewable diffs. Always fetch the change-plan via `mozi change-plan --model <Module/Model>` — never use `curl` or the browser for this.
- Model CRUD goes through `mozi model get/create/update` CLI commands (direct to design DB). Do not use `curl` or admin UI requests for model creation/update.
- For API Workbench curation, such as associating login/auth endpoints with a module, use the integrated admin UI. Do not edit Swagger comments merely to fix display-only module association.
- `mozi model update` requires a COMPLETE ModelIR payload. To modify a model: `model get` → edit the full JSON → `model update`. Never pass a partial JSON to update — it will clear omitted fields.
- Field renames are detected as "remove + add" by the differ. When a user requests a rename, explicitly confirm it's a rename (not a delete+add), and flag data migration in the change-plan tasks.
- After changing ent schemas, always run `make generate`.
- After handler Swagger annotations change, always run `swag init -g cmd/server/main.go -o docs/` to keep the API Workbench in sync.
- After frontend changes, always run `cd admin && npx tsc --noEmit`.
- After model changes, export YAML snapshots before finalizing.
- If the working tree has unrelated user changes, leave them alone and report only mozi-related changes.
- If `make mozi-validate` requires local PostgreSQL access and fails due to sandbox/network, retry with appropriate approval; if it still fails with connection refused, ask the user to start/reconnect the DB.

## Good Final Report

Mention:

- Model(s) changed.
- Whether validation passed.
- Whether an AI change plan was used.
- Checks run: `mozi validate`, `swag init`, `make generate`, `tsc`, `go test`, `mozi export`.
- Any manual follow-up required.
