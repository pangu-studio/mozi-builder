import React from 'react'
import { Table, Button, Space, Tag, Tooltip } from 'antd'
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import type { FieldIR } from '../api/dev-platform'

const FIELD_TYPE_COLORS: Record<string, string> = {
  string: 'blue',
  int: 'green',
  float: 'cyan',
  bool: 'purple',
  time: 'orange',
  text: 'geekblue',
  enum: 'magenta',
  json: 'gold',
}

const FORM_TYPE_LABELS: Record<string, string> = {
  text: '文本框',
  email: '邮箱',
  password: '密码',
  number: '数字',
  select: '下拉框',
  switch: '开关',
  date: '日期',
  textarea: '文本域',
  upload: '上传',
  richtext: '富文本',
}

interface FieldTableProps {
  fields: FieldIR[]
  onAdd: () => void
  onEdit: (field: FieldIR, index: number) => void
  onDelete: (field: FieldIR, index: number) => void
  onMove?: (fromIndex: number, toIndex: number) => void
  loading?: boolean
}

const FieldTable: React.FC<FieldTableProps> = ({
  fields,
  onAdd,
  onEdit,
  onDelete,
  onMove,
  loading,
}) => {
  const columns: ColumnsType<FieldIR & { _index: number }> = [
    {
      title: '字段名',
      dataIndex: 'name',
      key: 'name',
      width: 140,
      render: (v: string, record) => (
        <Space size={4}>
          {record.primary && <Tag color="red" style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>PK</Tag>}
          <span style={{ fontFamily: 'monospace' }}>{v}</span>
          {record.unique && <Tag style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>UK</Tag>}
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 80,
      render: (v: string) => <Tag color={FIELD_TYPE_COLORS[v] || 'default'}>{v}</Tag>,
    },
    {
      title: '标签',
      dataIndex: 'label',
      key: 'label',
      width: 120,
    },
    {
      title: '表单',
      dataIndex: 'form_type',
      key: 'form_type',
      width: 90,
      render: (v: string) => FORM_TYPE_LABELS[v] || v || 'text',
    },
    {
      title: '属性',
      key: 'attrs',
      width: 160,
      render: (_: unknown, record: FieldIR & { _index: number }) => (
        <Space size={4} wrap>
          {record.required && <Tag color="red">必填</Tag>}
          {record.searchable && <Tag color="green">可搜索</Tag>}
          {record.listable === false && <Tag>隐藏</Tag>}
          {record.editable === false && <Tag>只读</Tag>}
          {record.sensitive && <Tag color="orange">敏感</Tag>}
        </Space>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      render: (_: unknown, record: FieldIR & { _index: number }) => (
        <Space size={4}>
          {onMove && (
            <>
              <Tooltip title="上移">
                <Button
                  type="text"
                  size="small"
                  icon={<ArrowUpOutlined />}
                  disabled={record._index === 0}
                  onClick={() => onMove(record._index, record._index - 1)}
                />
              </Tooltip>
              <Tooltip title="下移">
                <Button
                  type="text"
                  size="small"
                  icon={<ArrowDownOutlined />}
                  disabled={record._index === fields.length - 1}
                  onClick={() => onMove(record._index, record._index + 1)}
                />
              </Tooltip>
            </>
          )}
          <Tooltip title="编辑">
            <Button
              type="text"
              size="small"
              icon={<EditOutlined />}
              onClick={() => onEdit(record, record._index)}
            />
          </Tooltip>
          <Tooltip title="删除">
            <Button
              type="text"
              size="small"
              danger
              icon={<DeleteOutlined />}
              onClick={() => onDelete(record, record._index)}
            />
          </Tooltip>
        </Space>
      ),
    },
  ]

  const dataSource = fields.map((f, i) => ({ ...f, _index: i, key: f.name || `field-${i}` }))

  return (
    <div>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <span style={{ fontWeight: 500, fontSize: 14 }}>字段列表</span>
        <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={onAdd}>
          添加字段
        </Button>
      </div>
      <Table
        columns={columns}
        dataSource={dataSource}
        loading={loading}
        size="small"
        pagination={false}
        locale={{ emptyText: '暂无字段，点击"添加字段"开始' }}
      />
    </div>
  )
}

export default FieldTable
