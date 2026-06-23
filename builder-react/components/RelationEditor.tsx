import React, { useEffect } from 'react'
import { Modal, Form, Input, Select, Switch } from 'antd'
import type { RelationIR, ModelSummary } from '../api/dev-platform'

interface RelationEditorProps {
  visible: boolean
  relation: RelationIR | null
  models: ModelSummary[]
  currentModule: string
  onOk: (relation: RelationIR) => void
  onCancel: () => void
}

const RELATION_TYPES = [
  { value: 'has_many', label: 'has_many — 拥有多个' },
  { value: 'has_one', label: 'has_one — 拥有一个' },
  { value: 'belongs_to', label: 'belongs_to — 属于' },
  { value: 'many_to_many', label: 'many_to_many — 多对多' },
]

const RelationEditor: React.FC<RelationEditorProps> = ({
  visible,
  relation,
  models,
  currentModule,
  onOk,
  onCancel,
}) => {
  const [form] = Form.useForm()
  const isEdit = relation !== null

  const targetOptions = models.map((m) => ({
    value: `${m.module}/${m.name}`,
    label: `${m.module}/${m.name} (${m.label})`,
  }))

  useEffect(() => {
    if (visible) {
      if (relation) {
        form.setFieldsValue({
          name: relation.name,
          label: relation.label || '',
          type: relation.type,
          target: relation.target_module
            ? `${relation.target_module}/${relation.target_model}`
            : relation.target || '',
          back_ref: relation.back_ref || '',
          cascade: relation.cascade || false,
        })
      } else {
        form.resetFields()
      }
    }
  }, [visible, relation, form])

  const handleOk = async () => {
    try {
      const values = await form.validateFields()
      const targetParts = values.target.split('/')
      const result: RelationIR = {
        name: values.name,
        label: values.label || '',
        type: values.type,
        target: values.target,
        target_module: targetParts.length > 1 ? targetParts[0] : currentModule,
        target_model: targetParts.length > 1 ? targetParts[1] : targetParts[0],
        back_ref: values.back_ref || '',
        cascade: values.cascade || false,
      }
      onOk(result)
    } catch {
      // 表单校验失败
    }
  }

  return (
    <Modal
      title={isEdit ? `编辑关系：${relation?.name}` : '新建关系'}
      open={visible}
      onOk={handleOk}
      onCancel={onCancel}
      width={500}
      destroyOnClose
    >
      <Form form={form} layout="vertical" initialValues={{ type: 'has_many' }} style={{ marginTop: 16 }}>
        <Form.Item
          name="name"
          label="关联名"
          rules={[{ required: true, message: '请输入关联名' }]}
        >
          <Input placeholder="代码中从当前模型访问目标模型，如 cards、owner" />
        </Form.Item>
        <Form.Item name="label" label="关系谓词">
          <Input placeholder="用于 ER 图展示，如 包含、归属于、创建、产生" />
        </Form.Item>
        <Form.Item
          name="type"
          label="关系类型"
          rules={[{ required: true }]}
        >
          <Select options={RELATION_TYPES} />
        </Form.Item>
        <Form.Item
          name="target"
          label="目标模型"
          rules={[{ required: true, message: '请选择目标模型' }]}
        >
          <Select
            options={targetOptions}
            showSearch
            placeholder="选择目标模型（模块/模型名）"
            filterOption={(input, option) =>
              (option?.label as string)?.toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>
        <Form.Item name="back_ref" label="反向导航属性">
          <Input placeholder="代码中从目标模型反向访问当前模型，如 deck、created_notes" />
        </Form.Item>
        <Form.Item name="cascade" valuePropName="checked" label="级联删除">
          <Switch />
        </Form.Item>
      </Form>
    </Modal>
  )
}

export default RelationEditor
