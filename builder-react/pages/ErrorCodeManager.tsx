import React, { useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import { DeleteOutlined, EditOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import {
  deleteErrorCode,
  listErrorCodes,
  saveErrorCode,
  type ErrorCodeIR,
} from '../api/dev-platform'

const { Title, Text } = Typography

const CATEGORY_OPTIONS = [
  { value: 'resource', label: '资源' },
  { value: 'validation', label: '参数校验' },
  { value: 'permission', label: '权限' },
  { value: 'business', label: '业务规则' },
  { value: 'auth', label: '认证' },
  { value: 'rate_limit', label: '限流' },
  { value: 'system', label: '系统' },
]

const emptyCode: ErrorCodeIR = {
  code: '',
  domain: '',
  http_status: 400,
  category: 'business',
  message: '',
  consumer_facing: true,
  retryable: false,
  details_schema: '',
  i18n_key: '',
  deprecated: false,
}

const ErrorCodeManager: React.FC = () => {
  const [items, setItems] = useState<ErrorCodeIR[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [open, setOpen] = useState(false)
  const [editingCode, setEditingCode] = useState<string | null>(null)
  const [keyword, setKeyword] = useState('')
  const [category, setCategory] = useState<string>()
  const [form] = Form.useForm<ErrorCodeIR>()

  const load = async () => {
    setLoading(true)
    try {
      const res = await listErrorCodes()
      setItems(res.data || [])
    } catch (err: any) {
      message.error(err?.response?.data?.error || err?.message || '加载错误码失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const filtered = useMemo(() => {
    const q = keyword.trim().toLowerCase()
    return items.filter((item) => {
      if (category && item.category !== category) return false
      if (!q) return true
      return [item.code, item.domain, item.message, item.i18n_key]
        .some((value) => value?.toLowerCase().includes(q))
    })
  }, [items, keyword, category])

  const openCreate = () => {
    setEditingCode(null)
    form.setFieldsValue(emptyCode)
    setOpen(true)
  }

  const openEdit = (item: ErrorCodeIR) => {
    setEditingCode(item.code)
    form.setFieldsValue(item)
    setOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    setSaving(true)
    try {
      await saveErrorCode({ ...emptyCode, ...values, code: editingCode || values.code.trim() })
      message.success(editingCode ? '错误码已更新' : '错误码已创建')
      setOpen(false)
      await load()
    } catch (err: any) {
      message.error(err?.response?.data?.error || err?.message || '保存错误码失败')
    } finally {
      setSaving(false)
    }
  }

  const remove = async (code: string) => {
    try {
      await deleteErrorCode(code)
      message.success(`已删除 ${code}`)
      await load()
    } catch (err: any) {
      message.error(err?.response?.data?.error || err?.message || '删除错误码失败')
    }
  }

  const columns: ColumnsType<ErrorCodeIR> = [
    {
      title: '错误码', dataIndex: 'code', width: 210,
      render: (value: string, record) => <Space><Text code>{value}</Text>{record.deprecated && <Tag>已废弃</Tag>}</Space>,
    },
    { title: '业务域', dataIndex: 'domain', width: 110, render: (value?: string) => value || <Text type="secondary">全局</Text> },
    { title: 'HTTP', dataIndex: 'http_status', width: 80, render: (value: number) => <Tag color={value >= 500 ? 'red' : value === 429 ? 'orange' : 'blue'}>{value}</Tag> },
    { title: '分类', dataIndex: 'category', width: 110, render: (value: string) => CATEGORY_OPTIONS.find((item) => item.value === value)?.label || value },
    { title: '默认消息', dataIndex: 'message', ellipsis: true },
    { title: '客户端可见', dataIndex: 'consumer_facing', width: 100, render: (value: boolean) => value ? <Tag color="green">是</Tag> : <Tag>否</Tag> },
    { title: '可重试', dataIndex: 'retryable', width: 80, render: (value: boolean) => value ? <Tag color="orange">是</Tag> : <Tag>否</Tag> },
    {
      title: '操作', key: 'actions', width: 110,
      render: (_, record) => (
        <Space size={2}>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
          <Popconfirm title={`删除错误码 ${record.code}？`} description="请先确认没有模型继续引用该错误码。" onConfirm={() => remove(record.code)}>
            <Button type="link" size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>错误码管理</Title>
        <Space>
          <Button icon={<ReloadOutlined />} loading={loading} onClick={load}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建错误码</Button>
        </Space>
      </div>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        title="项目级错误码注册表"
        description="API Intent 和测试合约只能引用这里登记的错误码。修改状态码或删除错误码可能构成破坏性变更。"
      />

      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Input.Search allowClear placeholder="搜索错误码、业务域或消息" style={{ width: 300 }} value={keyword} onChange={(e) => setKeyword(e.target.value)} />
          <Select allowClear placeholder="全部分类" style={{ width: 150 }} value={category} onChange={setCategory} options={CATEGORY_OPTIONS} />
          <Text type="secondary">共 {filtered.length} 项</Text>
        </Space>
        <Table columns={columns} dataSource={filtered} rowKey="code" loading={loading} pagination={{ pageSize: 20 }} />
      </Card>

      <Modal title={editingCode ? `编辑 ${editingCode}` : '新建错误码'} open={open} onCancel={() => setOpen(false)} onOk={submit} confirmLoading={saving} width={680} destroyOnHidden>
        <Form form={form} layout="vertical" initialValues={emptyCode}>
          <Form.Item name="code" label="错误码" rules={[{ required: true }, { pattern: /^[A-Z][A-Z0-9_]*$/, message: '使用大写字母、数字和下划线' }]}>
            <Input disabled={!!editingCode} placeholder="DECK_NOT_FOUND" />
          </Form.Item>
          <Space size={16} style={{ display: 'flex' }} align="start">
            <Form.Item name="domain" label="业务域" style={{ flex: 1 }}><Input placeholder="content；留空表示全局" /></Form.Item>
            <Form.Item name="http_status" label="HTTP 状态码" rules={[{ required: true }]}><InputNumber min={400} max={599} /></Form.Item>
            <Form.Item name="category" label="分类" rules={[{ required: true }]}><Select style={{ width: 150 }} options={CATEGORY_OPTIONS} /></Form.Item>
          </Space>
          <Form.Item name="message" label="默认消息" rules={[{ required: true }]}><Input.TextArea rows={2} /></Form.Item>
          <Space size={16} style={{ display: 'flex' }} align="start">
            <Form.Item name="details_schema" label="Details Schema" style={{ flex: 1 }}><Input placeholder="ResourceNotFoundDetails" /></Form.Item>
            <Form.Item name="i18n_key" label="i18n Key" style={{ flex: 1 }}><Input placeholder="error.deck.not_found" /></Form.Item>
          </Space>
          <Space size={32}>
            <Form.Item name="consumer_facing" label="客户端可见" valuePropName="checked"><Switch /></Form.Item>
            <Form.Item name="retryable" label="允许重试" valuePropName="checked"><Switch /></Form.Item>
            <Form.Item name="deprecated" label="标记废弃" valuePropName="checked"><Switch /></Form.Item>
          </Space>
        </Form>
      </Modal>
    </div>
  )
}

export default ErrorCodeManager
