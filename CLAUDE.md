# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

**Mozi** is a Model-Driven Development Platform. Business models are defined in YAML, versioned in a PostgreSQL design database, and code is applied via AI Coding change plans (not template overwrites). The platform targets Go/ent backends and React/Ant Design frontends.

```text
mozi-builder/
├── cmd/mozi/cmd/     # CLI (cobra): init, validate, diff, model, change-plan, sync, export, etc.
├── mozi/             # Core library — zero deps on external project code
│   ├── types.go      # Central IR types: ModelIR, FieldIR, RelationIR, etc.
│   ├── parser/       # YAML → ModelIR parsing + validation
│   ├── generator/    # Template engine ([[/]] delimiters, embedded fs.FS)
│   ├── differ/       # Field-level ModelIR diff (added/removed/modified)
│   ├── apply/        # Code generation plan + file writing
│   ├── db/           # PostgreSQL design database CRUD
│   ├── manifest/     # .mozi/manifest.json — tracks synced versions
│   └── templates/    # Embedded Go templates (backend/ + frontend/)
├── devplatform/      # Gin HTTP API for the visual dev platform
└── builder-react/    # React library (not a standalone app) — model designer UI
```

## Commands

```bash
# Build the CLI
go build ./cmd/mozi

# Run tests
go test ./...

# Run tests for a specific package
go test ./mozi/parser/...
go test ./devplatform/...

# CLI usage (from a business project directory)
mozi validate                  # Validate all model YAML
mozi diff --model content/Deck # Show field-level changes since last sync
mozi change-plan -m content/Deck # Generate AI coding contract
mozi export --module content   # Export module models as YAML snapshots
mozi sync --model content/Deck # Mark model as synced in manifest
mozi model list                # List all models
mozi history --model content/Deck # Show version history
```

## Core Architecture

### ModelIR is the Central Data Structure

Every component operates on `mozi.ModelIR` (defined in `mozi/types.go`). YAML is parsed into IR, templates render from IR, diffs compare two IRs, and the database stores IR snapshots. The IR has 8 field types (`string`, `int`, `float`, `bool`, `time`, `text`, `enum`, `json`) and 4 relation types (`has_one`, `has_many`, `belongs_to`, `many_to_many`).

### From YAML to Code: The Change Plan Flow

The old `mozi gen` (template overwrite) is **retired**. The current workflow is:

1. **Model change** — Model created/updated in the design database (via CLI `mozi model create/update` or dev platform API)
2. **Diff** — `differ.Compare(prevModel, currentModel)` produces a `DiffResult` with structured `FieldChange` entries
3. **Change plan** — The `Service.ChangePlan()` method assembles an AI Coding contract with: intent, semantics, UI/API intent, diff details, affected file paths, tasks by area, verification checks, and a structured text prompt
4. **AI agent applies patch** — An external AI Coding agent reads the change plan and applies an incremental, reviewable patch to the actual codebase

The `devplatform.Service` has no side effects on the host codebase — it only reads the design DB and produces plans.

### Template Engine

Uses Go's `text/template` with `[[`/`]]` delimiters (to avoid conflicting with client-side template syntax). Templates are embedded via `//go:embed all:templates` in `mozi/embed.go`. The `generator.Engine` is framework-agnostic — it only knows about `TemplateContext`. All framework-specific conventions (ent schema methods, Ant Design component patterns, route structure) live in the template files.

The `generator.TemplateContext` pre-computes derived values: name variants (PascalCase, snake_case, camelCase, plural), categorized fields (listable, editable, required, searchable), relation groups, and package paths.

### Design Database (PostgreSQL)

Separate from the application database. Stores: modules, models, version history with full YAML snapshots, field/relation/admin CRUD tables, design dictionaries, and API endpoint overrides. Every model save creates a timestamp-based version. `db.Store.LoadModelVersion()` prefers the YAML snapshot (which includes semantics/ui_intent/api_intent) and falls back to structured tables when no snapshot is available.

### Dev Platform API

`devplatform/routes.go` registers Gin routes. The `Service` wraps `db.Store` + `generator.Engine`. Routes handle: model CRUD, module CRUD, ER diagram generation (Mermaid DSL), validation, diff, change plan, API asset listing, and design dictionary management. Designed to be mounted into a host app's Gin router.

### React Builder Library

`builder-react/` is a **library**, not a standalone app. Exports:

- `BuilderRoutes` — React Router v7 route elements (models, apis, guide, er, model designer, diff viewer)
- `MoziBuilderProvider` + `useMoziBuilder` — context provider
- `createMoziBuilderMenuItem` — menu integration

Host apps mount these routes at a path of their choosing. Uses Zustand stores, Ant Design v6, and React Router v7.

### Marker System

Generated code uses marker comments for safe incremental updates: `// mozi:section Name` / `// mozi:end Name`, `// mozi:columns Name`, `// mozi:api Name`, `// mozi:route ref`, `// mozi:import ref`. The `generator.MarkerSection` type and `ReplaceMarkerSection` function handle extracting and replacing marked sections without touching surrounding hand-written code.

## Key Conventions

- All IDs are strings (typically UUIDs), even primary keys
- Module names are used as-is for directory names and API prefixes
- Model names are PascalCase; table names are snake_case plural
- The design database is the source of truth; YAML files in `models/` are export snapshots
- The manifest (`.mozi/manifest.json`) tracks which model versions have been applied to code — use `mozi sync` to update it
