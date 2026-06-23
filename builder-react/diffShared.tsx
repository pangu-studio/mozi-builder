import React from 'react'
import { Tag, Typography, Empty } from 'antd'
import type { DiffChange, DiffSummary } from './api/dev-platform'

const { Text } = Typography

// Change-type visual config shared by the diff viewer and version history.
export const CHANGE_TYPE_CONFIG: Record<string, { color: string; label: string; icon: string }> = {
  added: { color: 'green', label: '新增', icon: '+' },
  removed: { color: 'red', label: '删除', icon: '-' },
  modified: { color: 'orange', label: '修改', icon: '~' },
}

export const CATEGORY_LABELS: Record<string, string> = {
  field: '字段',
  relation: '关联',
  admin: '后台配置',
  meta: '模型元数据',
  semantics: '业务语义',
  ui_intent: 'UI 意图',
  api_intent: 'API 意图',
}

// ChangeCountBadges renders the compact "+N -N ~N" summary used in version rows.
export const ChangeCountBadges: React.FC<{ summary?: DiffSummary | null }> = ({ summary }) => {
  if (!summary || !summary.has_changes) {
    return <Text type="secondary">无变更</Text>
  }
  const counts = summary.counts || {}
  return (
    <span>
      {counts.added ? <Tag color="green" style={{ marginInlineEnd: 4 }}>+ {counts.added}</Tag> : null}
      {counts.removed ? <Tag color="red" style={{ marginInlineEnd: 4 }}>- {counts.removed}</Tag> : null}
      {counts.modified ? <Tag color="orange" style={{ marginInlineEnd: 0 }}>~ {counts.modified}</Tag> : null}
    </span>
  )
}

// ChangeItem renders a single structured change, identical to the diff viewer.
export const ChangeItem: React.FC<{ change: DiffChange; last?: boolean }> = ({ change, last }) => {
  const cfg = CHANGE_TYPE_CONFIG[change.type] || { color: 'default', label: change.type, icon: '' }
  return (
    <div
      style={{
        padding: '10px 12px',
        borderBottom: last ? 'none' : '1px solid #f0f0f0',
        display: 'flex',
        alignItems: 'flex-start',
        gap: 10,
      }}
    >
      <Tag color={cfg.color}>{cfg.icon}</Tag>
      <div style={{ flex: 1 }}>
        <div style={{ marginBottom: 4 }}>
          <Tag style={{ fontSize: 11 }}>{CATEGORY_LABELS[change.category] || change.category}</Tag>
          <Text strong>{change.name}</Text>
        </div>
        <Text type="secondary" style={{ fontSize: 13 }}>
          {change.detail}
        </Text>
        {change.old_value && (
          <div style={{ marginTop: 4 }}>
            <Text delete type="danger" style={{ fontSize: 12 }}>
              {change.old_value}
            </Text>
          </div>
        )}
        {change.new_value && (
          <div style={{ marginTop: 2 }}>
            <Text type="success" style={{ fontSize: 12 }}>
              → {change.new_value}
            </Text>
          </div>
        )}
      </div>
    </div>
  )
}

// ChangeList renders a full change list, used in expanded version rows.
export const ChangeList: React.FC<{ changes?: DiffChange[] }> = ({ changes }) => {
  if (!changes || changes.length === 0) {
    return <Empty description="无变更明细" image={Empty.PRESENTED_IMAGE_SIMPLE} />
  }
  return (
    <div style={{ paddingInline: 8 }}>
      {changes.map((change, i) => (
        <ChangeItem key={`${change.category}-${change.name}-${i}`} change={change} last={i === changes.length - 1} />
      ))}
    </div>
  )
}
