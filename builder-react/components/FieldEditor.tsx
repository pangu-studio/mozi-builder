import React, { useEffect, useState } from 'react'
import { Modal, Form, Input, Select, Switch, Space, Tag } from 'antd'
import type { FieldIR } from '../api/dev-platform'

interface FieldEditorProps {
  visible: boolean
  field: FieldIR | null
  onOk: (field: FieldIR) => void
  onCancel: () => void
}

const FIELD_TYPES = [
  { value: 'string', label: 'string — 字符串' },
  { value: 'int', label: 'int — 整数' },
  { value: 'float', label: 'float — 浮点数' },
  { value: 'bool', label: 'bool — 布尔值' },
  { value: 'time', label: 'time — 时间' },
  { value: 'text', label: 'text — 长文本' },
  { value: 'enum', label: 'enum — 枚举' },
  { value: 'json', label: 'json — JSON' },
]

const FORM_TYPES = [
  { value: 'text', label: '文本框' },
  { value: 'email', label: '邮箱输入框' },
  { value: 'password', label: '密码输入框' },
  { value: 'number', label: '数字输入框' },
  { value: 'select', label: '下拉选择框' },
  { value: 'switch', label: '开关' },
  { value: 'date', label: '日期选择器' },
  { value: 'textarea', label: '文本域' },
  { value: 'upload', label: '文件上传' },
]

const FieldEditor: React.FC<FieldEditorProps> = ({ visible, field, onOk, onCancel }) => {
  const [form] = Form.useForm()
  const [fieldType, setFieldType] = useState<string>('string')
  const isEdit = field !== null

  useEffect(() => {
    if (visible) {
      if (field) {
        form.setFieldsValue({
          ...field,
          enum_values_str: (field.enum_values || []).join(', '),
        })
        setFieldType(field.type || 'string')
      } else {
        form.resetFields()
        setFieldType('string')
      }
    }
  }, [visible, field, form])

  const handleOk = async () => {
    try {
      const values = await form.validateFields()
      const enumValues = values.enum_values_str
        ? values.enum_values_str.split(',').map((s: string) => s.trim()).filter(Boolean)
        : []

      const result: FieldIR = {
        name: values.name,
        type: values.type,
        label: values.label,
        required: values.required || false,
        unique: values.unique || false,
        sensitive: values.sensitive || false,
        searchable: values.searchable !== false,
        listable: values.listable !== false,
        editable: values.editable !== false,
        sortable: values.sortable || false,
        default: values.default || '',
        form_type: values.form_type || 'text',
        enum_values: values.type === 'enum' ? enumValues : [],
      }

      // 保留原有字段
      if (field) {
        result.primary = field.primary
        result.auto_now_add = field.auto_now_add
        result.auto_now = field.auto_now
        result.generated = field.generated
      }

      onOk(result)
    } catch {
      // 表单校验失败
    }
  }

  return (
    <Modal
      title={isEdit ? `编辑字段：${field?.name}` : '新建字段'}
      open={visible}
      onOk={handleOk}
      onCancel={onCancel}
      width={560}
      destroyOnClose
    >
      <Form form={form} layout="vertical" initialValues={{ type: 'string', form_type: 'text' }} style={{ marginTop: 16 }}>
        <Space style={{ width: '100%' }} size={16}>
          <Form.Item
            name="name"
            label="字段名"
            rules={[
              { required: true, message: '请输入字段名' },
              { pattern: /^[a-z][a-z0-9_]*$/, message: '小写字母开头，只含字母、数字、下划线' },
            ]}
            style={{ width: 220 }}
          >
            <Input placeholder="如 email, created_at" />
          </Form.Item>
          <Form.Item
            name="label"
            label="标签"
            rules={[{ required: true, message: '请输入标签' }]}
            style={{ width: 160 }}
          >
            <Input placeholder="如 邮箱" />
          </Form.Item>
        </Space>

        <Form.Item name="type" label="类型" rules={[{ required: true }]}>
          <Select options={FIELD_TYPES} onChange={(v: string) => setFieldType(v)} />
        </Form.Item>

        <Form.Item name="form_type" label="前端表单类型">
          <Select options={FORM_TYPES} />
        </Form.Item>

        {fieldType === 'enum' && (
          <Form.Item
            name="enum_values_str"
            label="枚举值"
            tooltip="多个值用英文逗号分隔，如 active, disabled, pending"
          >
            <Input placeholder="active, disabled" />
          </Form.Item>
        )}

        <Form.Item name="default" label="默认值">
          <Input placeholder="可选" />
        </Form.Item>

        <div style={{ borderTop: '1px solid #f0f0f0', paddingTop: 12, marginTop: 8 }}>
          <Space size={24} wrap>
            <Form.Item name="required" valuePropName="checked" style={{ marginBottom: 8 }}>
              <Switch checkedChildren="必填" unCheckedChildren="必填" />
            </Form.Item>
            <Form.Item name="unique" valuePropName="checked" style={{ marginBottom: 8 }}>
              <Switch checkedChildren="唯一" unCheckedChildren="唯一" />
            </Form.Item>
            <Form.Item name="sensitive" valuePropName="checked" style={{ marginBottom: 8 }}>
              <Switch checkedChildren="敏感" unCheckedChildren="敏感" />
            </Form.Item>
            <Form.Item name="searchable" valuePropName="checked" initialValue style={{ marginBottom: 8 }}>
              <Switch checkedChildren="可搜索" unCheckedChildren="可搜索" />
            </Form.Item>
            <Form.Item name="listable" valuePropName="checked" initialValue style={{ marginBottom: 8 }}>
              <Switch checkedChildren="列表可见" unCheckedChildren="列表可见" />
            </Form.Item>
            <Form.Item name="editable" valuePropName="checked" initialValue style={{ marginBottom: 8 }}>
              <Switch checkedChildren="可编辑" unCheckedChildren="可编辑" />
            </Form.Item>
            <Form.Item name="sortable" valuePropName="checked" style={{ marginBottom: 8 }}>
              <Switch checkedChildren="可排序" unCheckedChildren="可排序" />
            </Form.Item>
          </Space>
        </div>
      </Form>
    </Modal>
  )
}

export default FieldEditor
