import React from 'react'
import { Handle, Position, type Node, type NodeProps } from '@xyflow/react'
import type { ERGraphField } from '../api/dev-platform'

// 固定尺寸常量：elk 布局预估必须与实际渲染一致
export const NODE_WIDTH = 260
export const HEADER_HEIGHT = 36
export const ROW_HEIGHT = 28

export type ERTableDirection = 'LR' | 'TB'

export type ERTableNodeData = {
  name: string
  label: string
  table: string
  fields: ERGraphField[]
  direction: ERTableDirection
  focused?: boolean // 点选聚焦时高亮边框
}

export type ERTableNodeType = Node<ERTableNodeData, 'erTable'>

// 字段行徽标
const FieldBadge: React.FC<{ text: string; color: string }> = ({ text, color }) => (
  <span
    style={{
      fontSize: 10,
      lineHeight: '14px',
      padding: '0 4px',
      marginLeft: 4,
      border: `1px solid ${color}`,
      borderRadius: 4,
      color,
      flexShrink: 0,
    }}
  >
    {text}
  </span>
)

const handleStyle: React.CSSProperties = {
  width: 6,
  height: 6,
  background: '#667eea',
  border: 'none',
  opacity: 0.8,
}

const ERTableNode: React.FC<NodeProps<ERTableNodeType>> = ({ data }) => {
  const horizontal = data.direction !== 'TB'
  const sourcePosition = horizontal ? Position.Right : Position.Bottom
  const targetPosition = horizontal ? Position.Left : Position.Top

  return (
    <div
      style={{
        width: NODE_WIDTH,
        background: '#fff',
        border: data.focused ? '1.5px solid #667eea' : '1px solid #d9d9e3',
        borderRadius: 6,
        overflow: 'hidden',
        fontSize: 12,
        cursor: 'pointer',
        boxShadow: data.focused
          ? '0 0 0 3px rgba(102,126,234,0.25), 0 1px 4px rgba(0,0,0,0.08)'
          : '0 1px 4px rgba(0,0,0,0.08)',
      }}
    >
      {/* 表头：模型名 + Label，兼作表级连线锚点 */}
      <div
        style={{
          position: 'relative',
          height: HEADER_HEIGHT,
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          padding: '0 10px',
          background: '#667eea',
          color: '#fff',
        }}
      >
        <span
          style={{
            fontWeight: 600,
            fontSize: 13,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {data.name}
        </span>
        {data.label && (
          <span
            style={{
              fontSize: 11,
              opacity: 0.85,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {data.label}
          </span>
        )}
        <Handle id="t:t" type="target" position={targetPosition} style={handleStyle} isConnectable={false} />
        <Handle id="t:s" type="source" position={sourcePosition} style={handleStyle} isConnectable={false} />
      </div>

      {/* 字段行：左列类型，中间字段名，右侧约束徽标 */}
      {data.fields.map((field) => (
        <div
          key={field.name}
          style={{
            position: 'relative',
            height: ROW_HEIGHT,
            display: 'flex',
            alignItems: 'center',
            padding: '0 10px',
            borderTop: '1px solid #f0f0f5',
          }}
        >
          <span
            style={{
              width: 48,
              flexShrink: 0,
              color: '#999',
              fontSize: 11,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {field.type}
          </span>
          <span
            style={{
              flex: 1,
              minWidth: 0,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              color: '#333',
            }}
            title={field.label ? `${field.name}（${field.label}）` : field.name}
          >
            {field.name}
          </span>
          {field.primary && <FieldBadge text="PK" color="#faad14" />}
          {field.foreign_key && <FieldBadge text="FK" color="#c0c4d6" />}
          {field.unique && <FieldBadge text="UK" color="#667eea" />}
          {field.required && <FieldBadge text="NN" color="#999" />}
          <Handle
            id={`f:${field.name}:t`}
            type="target"
            position={targetPosition}
            style={handleStyle}
            isConnectable={false}
          />
          <Handle
            id={`f:${field.name}:s`}
            type="source"
            position={sourcePosition}
            style={handleStyle}
            isConnectable={false}
          />
        </div>
      ))}
    </div>
  )
}

export default ERTableNode
