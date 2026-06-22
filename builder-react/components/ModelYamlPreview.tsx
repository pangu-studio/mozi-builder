import React from 'react'
import { Typography, Empty } from 'antd'
import type { ModelIR } from '../api/dev-platform'

const { Text, Paragraph } = Typography

interface ModelYamlPreviewProps {
  model: ModelIR | null
}

function modelToYaml(model: ModelIR): string {
  const lines: string[] = []
  lines.push(`module: ${model.module}`)
  lines.push(`model: ${model.name}`)
  lines.push(`label: ${model.label}`)
  if (model.description) lines.push(`description: ${model.description}`)
  lines.push(`table: ${model.table}`)
  lines.push('')

  if (model.semantics && hasSemanticContent(model.semantics)) {
    lines.push('semantics:')
    if (model.semantics.purpose) lines.push(`  purpose: ${quote(model.semantics.purpose)}`)
    if (model.semantics.audience?.length) lines.push(`  audience: [${model.semantics.audience.map(quote).join(', ')}]`)
    if (model.semantics.user_value) lines.push(`  user_value: ${quote(model.semantics.user_value)}`)
    pushStringList(lines, 'business_rules', model.semantics.business_rules)
    pushStringList(lines, 'permissions', model.semantics.permissions)
    pushStringList(lines, 'lifecycle', model.semantics.lifecycle)
    lines.push('')
  }

  if (model.ui_intent && hasUIIntentContent(model.ui_intent)) {
    lines.push('ui_intent:')
    if (model.ui_intent.product_goal) lines.push(`  product_goal: ${quote(model.ui_intent.product_goal)}`)
    if (model.ui_intent.user_tasks?.length) {
      lines.push('  user_tasks:')
      for (const task of model.ui_intent.user_tasks) {
        lines.push(`    - key: ${task.key || ''}`)
        if (task.label) lines.push(`      label: ${quote(task.label)}`)
        if (task.priority) lines.push(`      priority: ${task.priority}`)
      }
    }
    if (model.ui_intent.shared && hasSharedUIContent(model.ui_intent.shared)) {
      lines.push('  shared:')
      if (model.ui_intent.shared.primary_entities?.length)
        lines.push(`    primary_entities: [${model.ui_intent.shared.primary_entities.map(quote).join(', ')}]`)
      if (model.ui_intent.shared.primary_actions?.length)
        lines.push(`    primary_actions: [${model.ui_intent.shared.primary_actions.map(quote).join(', ')}]`)
      if (model.ui_intent.shared.empty_state)
        lines.push(`    empty_state: ${quote(model.ui_intent.shared.empty_state)}`)
      if (model.ui_intent.shared.terminology && Object.keys(model.ui_intent.shared.terminology).length > 0) {
        lines.push('    terminology:')
        for (const [key, value] of Object.entries(model.ui_intent.shared.terminology)) {
          lines.push(`      ${key}: ${quote(value)}`)
        }
      }
    }
    if (model.ui_intent.surfaces_config && Object.keys(model.ui_intent.surfaces_config).length > 0) {
      lines.push('  surfaces_config:')
      for (const [surface, config] of Object.entries(model.ui_intent.surfaces_config)) {
        lines.push(`    ${surface}:`)
        if (config.role) lines.push(`      role: ${quote(config.role)}`)
        if (config.enabled_tasks?.length)
          lines.push(`      enabled_tasks: [${config.enabled_tasks.map(quote).join(', ')}]`)
        if (config.views && Object.keys(config.views).length > 0) {
          lines.push('      views:')
          for (const [view, viewConfig] of Object.entries(config.views)) {
            lines.push(`        ${view}:`)
            if (viewConfig.intent) lines.push(`          intent: ${quote(viewConfig.intent)}`)
            if (viewConfig.density) lines.push(`          density: ${viewConfig.density}`)
            if (viewConfig.fields?.length)
              lines.push(`          fields: [${viewConfig.fields.map(quote).join(', ')}]`)
          }
        }
        if (config.actions?.length) lines.push(`      actions: [${config.actions.map(quote).join(', ')}]`)
        pushNestedStringList(lines, 'constraints', config.constraints, 6)
      }
    }
    if (model.ui_intent.surfaces?.length) lines.push(`  surfaces: [${model.ui_intent.surfaces.map(quote).join(', ')}]`)
    if (model.ui_intent.primary_view) lines.push(`  primary_view: ${model.ui_intent.primary_view}`)
    if (model.ui_intent.primary_actions?.length) lines.push(`  primary_actions: [${model.ui_intent.primary_actions.map(quote).join(', ')}]`)
    if (model.ui_intent.list_intent) lines.push(`  list_intent: ${quote(model.ui_intent.list_intent)}`)
    if (model.ui_intent.form_intent) lines.push(`  form_intent: ${quote(model.ui_intent.form_intent)}`)
    if (model.ui_intent.detail_intent) lines.push(`  detail_intent: ${quote(model.ui_intent.detail_intent)}`)
    if (model.ui_intent.empty_state) lines.push(`  empty_state: ${quote(model.ui_intent.empty_state)}`)
    pushStringList(lines, 'interaction_notes', model.ui_intent.interaction_notes)
    pushStringList(lines, 'surface_notes', model.ui_intent.surface_notes)
    lines.push('')
  }

  if (model.api_intent && hasAPIIntentContent(model.api_intent)) {
    lines.push('api_intent:')
    if (model.api_intent.exposure) lines.push(`  exposure: ${model.api_intent.exposure}`)
    if (model.api_intent.consumers?.length) lines.push(`  consumers: [${model.api_intent.consumers.map(quote).join(', ')}]`)
    if (model.api_intent.auth) lines.push(`  auth: ${quote(model.api_intent.auth)}`)
    if (model.api_intent.base_path) lines.push(`  base_path: ${quote(model.api_intent.base_path)}`)
    if (model.api_intent.operations?.length) lines.push(`  operations: [${model.api_intent.operations.map(quote).join(', ')}]`)
    pushStringList(lines, 'request_notes', model.api_intent.request_notes)
    pushStringList(lines, 'response_notes', model.api_intent.response_notes)
    pushStringList(lines, 'error_cases', model.api_intent.error_cases)
    if (model.api_intent.idempotency) lines.push(`  idempotency: ${quote(model.api_intent.idempotency)}`)
    if (model.api_intent.rate_limit) lines.push(`  rate_limit: ${quote(model.api_intent.rate_limit)}`)
    if (model.api_intent.versioning) lines.push(`  versioning: ${quote(model.api_intent.versioning)}`)
    pushStringList(lines, 'compatibility_notes', model.api_intent.compatibility_notes)
    lines.push('')
  }

  // 字段
  lines.push('fields:')
  for (const f of model.fields || []) {
    lines.push(`  - name: ${f.name}`)
    lines.push(`    type: ${f.type}`)
    lines.push(`    label: ${f.label}`)
    if (f.required) lines.push('    required: true')
    if (f.unique) lines.push('    unique: true')
    if (f.sensitive) lines.push('    sensitive: true')
    if (f.searchable) lines.push('    searchable: true')
    if (f.listable === false) lines.push('    listable: false')
    if (f.editable === false) lines.push('    editable: false')
    if (f.primary) lines.push('    primary: true')
    if (f.default) lines.push(`    default: "${f.default}"`)
    if (f.form_type && f.form_type !== 'text') lines.push(`    form_type: ${f.form_type}`)
    if (f.enum_values && f.enum_values.length > 0)
      lines.push(`    enum_values: [${f.enum_values.join(', ')}]`)
    if (f.auto_now_add) lines.push('    auto_now_add: true')
    if (f.auto_now) lines.push('    auto_now: true')
    if (f.generated && f.generated !== 'manual') lines.push(`    generated: ${f.generated}`)
    lines.push('')
  }

  // 关联
  if (model.relations && model.relations.length > 0) {
    lines.push('relations:')
    for (const r of model.relations) {
      lines.push(`  - name: ${r.name}`)
      lines.push(`    type: ${r.type}`)
      lines.push(`    target: ${r.target_module || model.module}/${r.target_model || r.target}`)
      if (r.back_ref) lines.push(`    back_ref: ${r.back_ref}`)
      if (r.cascade) lines.push('    cascade: true')
      lines.push('')
    }
  }

  // 后台配置
  if (model.admin) {
    const a = model.admin
    lines.push('admin:')
    if (a.list_columns?.length) lines.push(`  list_columns: [${a.list_columns.join(', ')}]`)
    if (a.search_fields?.length) lines.push(`  search_fields: [${a.search_fields.join(', ')}]`)
    lines.push(`  default_sort: ${a.default_sort || 'created_at'}`)
    lines.push(`  default_order: ${a.default_order || 'desc'}`)
    lines.push(`  page_size: ${a.page_size || 20}`)
    lines.push('')
  }

  return lines.join('\n')
}

function quote(value: string): string {
  return JSON.stringify(value)
}

function pushStringList(lines: string[], key: string, values?: string[]) {
  if (!values?.length) return
  lines.push(`  ${key}:`)
  for (const value of values) {
    lines.push(`    - ${quote(value)}`)
  }
}

function pushNestedStringList(lines: string[], key: string, values: string[] | undefined, indent: number) {
  if (!values?.length) return
  const pad = ' '.repeat(indent)
  lines.push(`${pad}${key}:`)
  for (const value of values) {
    lines.push(`${pad}  - ${quote(value)}`)
  }
}

function hasSemanticContent(semantics: NonNullable<ModelIR['semantics']>): boolean {
  return Boolean(
    semantics.purpose ||
    semantics.user_value ||
    semantics.audience?.length ||
    semantics.business_rules?.length ||
    semantics.permissions?.length ||
    semantics.lifecycle?.length,
  )
}

function hasUIIntentContent(uiIntent: NonNullable<ModelIR['ui_intent']>): boolean {
  return Boolean(
    uiIntent.product_goal ||
    uiIntent.user_tasks?.length ||
    (uiIntent.shared && hasSharedUIContent(uiIntent.shared)) ||
    (uiIntent.surfaces_config && Object.keys(uiIntent.surfaces_config).length > 0) ||
    uiIntent.primary_view ||
    uiIntent.surfaces?.length ||
    uiIntent.primary_actions?.length ||
    uiIntent.list_intent ||
    uiIntent.form_intent ||
    uiIntent.detail_intent ||
    uiIntent.empty_state ||
    uiIntent.interaction_notes?.length ||
    uiIntent.surface_notes?.length,
  )
}

function hasSharedUIContent(shared: NonNullable<ModelIR['ui_intent']>['shared']): boolean {
  return Boolean(
    shared?.primary_entities?.length ||
    shared?.primary_actions?.length ||
    shared?.empty_state ||
    (shared?.terminology && Object.keys(shared.terminology).length > 0),
  )
}

function hasAPIIntentContent(apiIntent: NonNullable<ModelIR['api_intent']>): boolean {
  return Boolean(
    apiIntent.exposure ||
    apiIntent.consumers?.length ||
    apiIntent.auth ||
    apiIntent.base_path ||
    apiIntent.operations?.length ||
    apiIntent.request_notes?.length ||
    apiIntent.response_notes?.length ||
    apiIntent.error_cases?.length ||
    apiIntent.idempotency ||
    apiIntent.rate_limit ||
    apiIntent.versioning ||
    apiIntent.compatibility_notes?.length,
  )
}

const ModelYamlPreview: React.FC<ModelYamlPreviewProps> = ({ model }) => {
  if (!model) {
    return (
      <div style={{ padding: 16 }}>
        <Empty description="暂无模型数据" />
      </div>
    )
  }

  const yaml = modelToYaml(model)

  return (
    <div style={{ height: '100%', overflow: 'auto' }}>
      <div style={{ marginBottom: 8 }}>
        <Text type="secondary" style={{ fontSize: 12, textTransform: 'uppercase', letterSpacing: 1 }}>
          YAML 实时预览
        </Text>
      </div>
      <pre
        style={{
          background: '#f6f8fa',
          borderRadius: 6,
          padding: 16,
          fontSize: 13,
          fontFamily: "'SF Mono', 'Monaco', 'Menlo', 'Courier New', monospace",
          lineHeight: 1.7,
          overflow: 'auto',
          margin: 0,
          border: '1px solid #e8e8e8',
        }}
      >
        {yaml}
      </pre>
    </div>
  )
}

export default ModelYamlPreview
