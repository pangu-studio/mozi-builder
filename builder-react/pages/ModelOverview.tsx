import React, { useEffect, useState } from 'react'
import { Alert, Table, Typography, Button, Space, Tag, message, Popconfirm, Card, Tooltip, Modal, Form, Input, Select } from 'antd'
import dayjs from 'dayjs'
import { PlusOutlined, ReloadOutlined, EditOutlined, DeleteOutlined, DiffOutlined, CheckCircleOutlined, AppstoreOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import type { ColumnsType } from 'antd/es/table'
import { useDevPlatformStore } from '../stores/dev-platform'
import type { ModelSummary, ModuleSummary } from '../api/dev-platform'
import { useMoziBuilder } from '..'

const { Title, Text } = Typography

const SYNC_STATUS_MAP: Record<string, { color: string; label: string }> = {
  synced: { color: 'green', label: '已同步' },
  modified: { color: 'orange', label: '有变更' },
  new: { color: 'blue', label: '新建' },
}

const ModelOverview: React.FC = () => {
  const navigate = useNavigate()
  const { buildRoute } = useMoziBuilder()
  const {
    modules,
    loading,
    error,
    loadModules,
    removeModel,
    createNewModule,
    saveModule,
    removeModule,
    clearError,
  } = useDevPlatformStore()
  const [removing, setRemoving] = useState<string | null>(null)
  const [selectedModule, setSelectedModule] = useState<string | undefined>(undefined)
  const [moduleModalOpen, setModuleModalOpen] = useState(false)
  const [editingModule, setEditingModule] = useState<ModuleSummary | null>(null)
  const [moduleSaving, setModuleSaving] = useState(false)
  const [removingModule, setRemovingModule] = useState<string | null>(null)
  const [moduleForm] = Form.useForm<ModuleSummary>()

  useEffect(() => {
    loadModules()
  }, [])

  // 错误提示
  useEffect(() => {
    if (error) {
      // 提取错误信息中关键部分展示
      const errMsg = error.split('\n')[0]
      message.error(errMsg)
      clearError()
    }
  }, [error])

  const handleDelete = async (modelSummary: ModelSummary) => {
    const fullName = `${modelSummary.module}/${modelSummary.name}`
    setRemoving(fullName)
    try {
      await removeModel(modelSummary.module, modelSummary.name)
      message.success(`已删除模型 ${modelSummary.name}`)
      await loadModules()
    } catch {
      message.error('删除失败')
    } finally {
      setRemoving(null)
    }
  }

  const handleRefresh = async () => {
    await loadModules()
    message.success('已刷新')
  }

  const openCreateModule = () => {
    setEditingModule(null)
    moduleForm.setFieldsValue({
      name: '',
      label: '',
      description: '',
      icon: '',
      api_prefix: '',
    })
    setModuleModalOpen(true)
  }

  const openEditModule = (module: ModuleSummary) => {
    setEditingModule(module)
    moduleForm.setFieldsValue({
      name: module.name,
      label: module.label,
      description: module.description,
      icon: module.icon,
      api_prefix: module.api_prefix,
    })
    setModuleModalOpen(true)
  }

  const handleSaveModule = async () => {
    const values = await moduleForm.validateFields()
    setModuleSaving(true)
    try {
      if (editingModule) {
        await saveModule(editingModule.name, values)
        message.success(`已更新模块 ${editingModule.name}`)
      } else {
        await createNewModule(values)
        message.success(`已创建模块 ${values.name}`)
        setSelectedModule(values.name)
      }
      setModuleModalOpen(false)
      await loadModules()
    } catch {
      message.error(editingModule ? '保存模块失败' : '创建模块失败')
    } finally {
      setModuleSaving(false)
    }
  }

  const handleDeleteModule = async (module: ModuleSummary) => {
    setRemovingModule(module.name)
    try {
      await removeModule(module.name)
      message.success(`已删除模块 ${module.name}`)
      if (selectedModule === module.name) {
        setSelectedModule(undefined)
      }
      await loadModules()
    } catch {
      message.error('删除模块失败')
    } finally {
      setRemovingModule(null)
    }
  }

  // 展平所有模型
  const allModels: (ModelSummary & { _module: string })[] = []
  for (const mod of modules) {
    for (const m of mod.models || []) {
      allModels.push({ ...m, _module: mod.name })
    }
  }
  const filteredModels = selectedModule
    ? allModels.filter((model) => model._module === selectedModule)
    : allModels

  const moduleColumns: ColumnsType<ModuleSummary> = [
    {
      title: '模块',
      dataIndex: 'name',
      key: 'name',
      width: 140,
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: '显示名',
      dataIndex: 'label',
      key: 'label',
      width: 140,
    },
    {
      title: 'API 前缀',
      dataIndex: 'api_prefix',
      key: 'api_prefix',
      width: 140,
      render: (v: string) => <Text code>/{v}</Text>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (v?: string) => v || <Text type="secondary">-</Text>,
    },
    {
      title: '模型数',
      dataIndex: 'model_count',
      key: 'model_count',
      width: 90,
      align: 'center',
    },
    {
      title: '操作',
      key: 'actions',
      width: 250,
      render: (_: unknown, record: ModuleSummary) => {
        const hasModels = (record.model_count || 0) > 0
        return (
          <Space size={4}>
            <Button type="link" size="small" onClick={() => setSelectedModule(record.name)}>
              查看模型
            </Button>
            <Button
              type="link"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => navigate(buildRoute('/models/new'), { state: { module: record.name } })}
            >
              补录模型
            </Button>
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => openEditModule(record)}
            >
              编辑
            </Button>
            <Tooltip title={hasModels ? '模块下存在模型，需先删除或迁移模型' : ''}>
              <span>
                <Popconfirm
                  title={`确定删除模块 ${record.name}？`}
                  description="只能删除空模块"
                  onConfirm={() => handleDeleteModule(record)}
                  okText="确定"
                  cancelText="取消"
                  disabled={hasModels}
                >
                  <Button
                    type="link"
                    size="small"
                    danger
                    icon={<DeleteOutlined />}
                    disabled={hasModels}
                    loading={removingModule === record.name}
                  />
                </Popconfirm>
              </span>
            </Tooltip>
          </Space>
        )
      },
    },
  ]

  const columns: ColumnsType<ModelSummary & { _module: string }> = [
    {
      title: '模块',
      dataIndex: '_module',
      key: 'module',
      width: 100,
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: '模型',
      dataIndex: 'name',
      key: 'name',
      width: 140,
      render: (v: string) => <span style={{ fontFamily: 'monospace', fontWeight: 500 }}>{v}</span>,
    },
    {
      title: '标签',
      dataIndex: 'label',
      key: 'label',
      width: 120,
    },
    {
      title: '表名',
      dataIndex: 'table',
      key: 'table',
      width: 130,
      render: (v: string) => <Text code>{v}</Text>,
    },
    {
      title: '字段',
      dataIndex: 'field_count',
      key: 'field_count',
      width: 70,
      align: 'center',
    },
    {
      title: '关联',
      dataIndex: 'rel_count',
      key: 'rel_count',
      width: 70,
      align: 'center',
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 160,
      align: 'center',
      render: (v: string) => {
        const t = dayjs(v, 'YYYYMMDDHHmmss')
        return t.isValid() ? (
          <Tooltip title={v}>
            <Tag>{t.format('YYYY-MM-DD HH:mm:ss')}</Tag>
          </Tooltip>
        ) : (
          <Tag>{v}</Tag>
        )
      },
    },
    {
      title: '状态',
      dataIndex: 'sync_status',
      key: 'sync_status',
      width: 90,
      render: (v: string) => {
        const s = SYNC_STATUS_MAP[v] || { color: 'default', label: v }
        return <Tag color={s.color}>{s.label}</Tag>
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 200,
      render: (_: unknown, record: ModelSummary & { _module: string }) => {
        const fullName = `${record._module}/${record.name}`
        return (
          <Space size={4}>
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => navigate(buildRoute(`/modules/${record._module}/models/${record.name}`))}
            >
              编辑
            </Button>
            {record.sync_status === 'synced' ? (
              <Tooltip title="当前模型版本已同步到代码，无待处理 AI 计划">
                <span>
                  <Button type="link" size="small" icon={<CheckCircleOutlined />} disabled>
                    已同步
                  </Button>
                </span>
              </Tooltip>
            ) : (
              <Button
                type="link"
                size="small"
                icon={<DiffOutlined />}
                onClick={() => navigate(buildRoute(`/modules/${record._module}/models/${record.name}/diff`))}
              >
                AI 计划
              </Button>
            )}
            <Popconfirm
              title={`确定删除模型 ${record.name}？`}
              description="此操作会同时删除所有版本历史和关联配置"
              onConfirm={() => handleDelete(record)}
              okText="确定"
              cancelText="取消"
            >
              <Button
                type="link"
                size="small"
                danger
                icon={<DeleteOutlined />}
                loading={removing === fullName}
              />
            </Popconfirm>
          </Space>
        )
      },
    },
  ]

  return (
    <div>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: 24,
        }}
      >
        <Title level={4} style={{ margin: 0 }}>
          模型管理
        </Title>
        <Space>
          <Select
            style={{ width: 180 }}
            placeholder="全部模块"
            allowClear
            value={selectedModule}
            onChange={setSelectedModule}
            options={modules.map((m) => ({
              label: `${m.label} (${m.name})`,
              value: m.name,
            }))}
          />
          <Button icon={<ReloadOutlined />} onClick={handleRefresh} loading={loading}>
            刷新
          </Button>
          <Button icon={<AppstoreOutlined />} onClick={openCreateModule}>
            新建模块
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => navigate(buildRoute('/models/new'), { state: { module: selectedModule } })}
          >
            补录模型
          </Button>
        </Space>
      </div>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="模型主线由 AI agent 与人的对话维护"
        description="管理后台用于查看模型、确认差异和少量微调。需要从零梳理模型时，优先在 Codex/Claude Code 会话中描述业务目标，让 Agent 按模型关注点生成草案。"
      />

      <Card
        title={`模块管理（${modules.length}）`}
        style={{ marginBottom: 16 }}
        extra={
          selectedModule ? (
            <Button type="link" onClick={() => setSelectedModule(undefined)}>
              查看全部模型
            </Button>
          ) : null
        }
      >
        <Table
          columns={moduleColumns}
          dataSource={modules}
          loading={loading}
          rowKey={(r) => r.name}
          size="middle"
          pagination={false}
          locale={{ emptyText: '暂无模块，点击右上角"新建模块"开始' }}
        />
      </Card>

      {/* 模型列表 */}
      <Card title={`模型列表（${filteredModels.length}）${selectedModule ? ` / ${selectedModule}` : ''}`}>
        <Table
          columns={columns}
          dataSource={filteredModels}
          loading={loading}
          rowKey={(r) => `${r._module}/${r.name}`}
          size="middle"
          pagination={false}
          locale={{ emptyText: '暂无模型，建议先通过 AI agent 对话建模，或点击右上角"补录模型"' }}
        />
      </Card>

      <Modal
        title={editingModule ? `编辑模块 ${editingModule.name}` : '新建模块'}
        open={moduleModalOpen}
        onOk={handleSaveModule}
        onCancel={() => setModuleModalOpen(false)}
        confirmLoading={moduleSaving}
        okText="保存"
        cancelText="取消"
      >
        <Form form={moduleForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="name"
            label="模块名"
            rules={[
              { required: true, message: '请输入模块名' },
              { pattern: /^[A-Za-z0-9_-]+$/, message: '只能使用字母、数字、下划线或短横线' },
            ]}
          >
            <Input disabled={!!editingModule} placeholder="content" />
          </Form.Item>
          <Form.Item name="label" label="显示名">
            <Input placeholder="内容" />
          </Form.Item>
          <Form.Item name="api_prefix" label="API 前缀">
            <Input addonBefore="/" placeholder="content" />
          </Form.Item>
          <Form.Item name="icon" label="图标">
            <Input placeholder="可选，如 book" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="模块用途说明" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ModelOverview
