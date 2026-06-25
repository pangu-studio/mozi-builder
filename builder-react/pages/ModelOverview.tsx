import React, { useEffect, useState } from 'react'
import {
  Alert,
  Table,
  Typography,
  Button,
  Space,
  Tag,
  message,
  Popconfirm,
  Card,
  Collapse,
  Tooltip,
  Modal,
  Form,
  Input,
  Select,
  Drawer,
  Descriptions,
  Empty,
} from 'antd'
import dayjs from 'dayjs'
import {
  PlusOutlined,
  ReloadOutlined,
  EditOutlined,
  DeleteOutlined,
  DiffOutlined,
  CheckCircleOutlined,
  AppstoreOutlined,
  EyeOutlined,
  HistoryOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import type { ColumnsType } from 'antd/es/table'
import { useDevPlatformStore } from '../stores/dev-platform'
import { getModel, getModelHistory } from '../api/dev-platform'
import type { FieldIR, ModelIR, ModelSummary, ModelVersionInfo, ModuleSummary, RelationIR, UIIntentConfig, UISurfaceIntentConfig, UISurfaceViewConfig } from '../api/dev-platform'
import { ChangeCountBadges, ChangeList } from '../diffShared'
import { useMoziBuilder } from '..'
import IconSelect from '../components/IconSelect'

const { Title, Text } = Typography

const SYNC_STATUS_MAP: Record<string, { color: string; label: string }> = {
  synced: { color: 'green', label: '已同步' },
  modified: { color: 'orange', label: '有变更' },
  new: { color: 'blue', label: '新建' },
}

const formatVersionTime = (version: string) => {
  const base = version.includes('_') ? version.slice(0, version.lastIndexOf('_')) : version
  const t = dayjs(base, 'YYYYMMDDHHmmss')
  return t.isValid() ? t.format('YYYY-MM-DD HH:mm:ss') : version
}

const formatDateTime = (value?: string) => {
  if (!value) return '-'
  const t = dayjs(value)
  return t.isValid() ? t.format('YYYY-MM-DD HH:mm:ss') : value
}

const renderBool = (value?: boolean) => (value ? <Tag color="green">是</Tag> : <Text type="secondary">否</Text>)

const renderTextList = (values?: string[]) => {
  const items = (values || []).filter(Boolean)
  return items.length ? items.join('、') : <Text type="secondary">-</Text>
}

const renderTagList = (values?: string[]) => {
  const items = (values || []).filter(Boolean)
  return items.length ? (
    <Space size={[4, 4]} wrap>
      {items.map((item) => (
        <Tag key={item}>{item}</Tag>
      ))}
    </Space>
  ) : (
    <Text type="secondary">-</Text>
  )
}

// === UI Intent surface labels (mirrors ModelDesigner.tsx Collapse labels) ===
const SURFACE_LABELS: Record<string, string> = {
  admin: '管理后台',
  desktop: '桌面客户端',
  miniapp: '小程序',
}

const DENSITY_LABELS: Record<string, string> = {
  high: '高密度',
  medium: '中密度',
  low: '低密度',
}

// === UI Intent rendering helpers ===

const hasUIIntent = (ui?: UIIntentConfig): boolean => {
  if (!ui) return false
  if (ui.product_goal?.trim()) return true
  if ((ui.user_tasks || []).some(t => t.key || t.label || t.priority)) return true
  const s = ui.shared
  if (s) {
    if ((s.primary_entities || []).length) return true
    if ((s.primary_actions || []).length) return true
    if (s.empty_state?.trim()) return true
    if (Object.keys(s.terminology || {}).length) return true
  }
  if (ui.surfaces_config && Object.keys(ui.surfaces_config).length) return true
  // legacy fields
  if ((ui.surfaces || []).length) return true
  if (ui.primary_view?.trim()) return true
  if (ui.primary_actions?.length) return true
  if (ui.list_intent?.trim()) return true
  if (ui.form_intent?.trim()) return true
  if (ui.detail_intent?.trim()) return true
  if (ui.empty_state?.trim()) return true
  if ((ui.interaction_notes || []).length) return true
  if ((ui.surface_notes || []).length) return true
  return false
}

const renderDensity = (v?: string) => {
  if (!v) return <Text type="secondary">-</Text>
  return <Tag>{DENSITY_LABELS[v] || v}</Tag>
}

const renderTerminology = (term?: Record<string, string>) => {
  const entries = Object.entries(term || {}).filter(([, v]) => v)
  if (!entries.length) return <Text type="secondary">-</Text>
  return (
    <Space direction="vertical" size={2}>
      {entries.map(([k, v]) => (
        <span key={k}>
          <Text code>{k}</Text>: {v}
        </span>
      ))}
    </Space>
  )
}

const renderSurfaceView = (name: string, view?: UISurfaceViewConfig) => {
  if (!view || (!view.intent?.trim() && !view.density && !(view.fields || []).length)) return null
  const label = name === 'list' ? '列表视图' : name === 'detail' ? '详情视图' : name === 'form' ? '表单视图' : name + '视图'
  return (
    <Descriptions
      key={name}
      bordered
      size="small"
      column={1}
      title={label}
      items={[
        { label: '视图意图', children: view.intent?.trim() || <Text type="secondary">-</Text> },
        { label: '密度', children: renderDensity(view.density) },
        { label: '字段', children: renderTagList(view.fields) },
      ]}
    />
  )
}

const renderSurfacePanel = (surface: string, cfg: UISurfaceIntentConfig) => {
  const hasContent = cfg.role?.trim() ||
    (cfg.enabled_tasks || []).length ||
    (cfg.actions || []).length ||
    (cfg.constraints || []).length ||
    Object.values(cfg.views || {}).some(v => v.intent?.trim() || v.density || (v.fields || []).length)
  if (!hasContent) return <Text type="secondary">暂无配置</Text>

  const views = cfg.views || {}
  const orderedViewKeys = ['list', 'detail', 'form']
  const extraViewKeys = Object.keys(views).filter(k => !orderedViewKeys.includes(k))

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Descriptions
        bordered
        size="small"
        column={1}
        items={[
          { label: '端侧角色', children: cfg.role?.trim() || <Text type="secondary">-</Text> },
          { label: '启用任务', children: renderTagList(cfg.enabled_tasks) },
          { label: '端侧动作', children: renderTagList(cfg.actions) },
          { label: '端侧约束', children: renderTextList(cfg.constraints) },
        ]}
      />
      {orderedViewKeys.map(k => {
        const v = views[k]
        return v ? renderSurfaceView(k, v) : null
      })}
      {extraViewKeys.map(k => {
        const v = views[k]
        return v ? renderSurfaceView(k, v) : null
      })}
    </Space>
  )
}

const renderUIIntent = (ui: UIIntentConfig) => {
  const surfacesConfig = ui.surfaces_config || {}
  const surfaceKeys = Object.keys(surfacesConfig)
  const ordered = ['admin', 'desktop', 'miniapp'].filter(k => surfaceKeys.includes(k))
  const extra = surfaceKeys.filter(k => !ordered.includes(k))
  const orderedKeys = [...ordered, ...extra]

  const taskColumns: ColumnsType<{ key?: string; label?: string; priority?: string }> = [
    { title: '标识', dataIndex: 'key', key: 'key', width: 140, render: (v?: string) => v ? <Text code>{v}</Text> : <Text type="secondary">-</Text> },
    { title: '名称', dataIndex: 'label', key: 'label', width: 140, render: (v?: string) => v || <Text type="secondary">-</Text> },
    { title: '优先级', dataIndex: 'priority', key: 'priority', width: 100, render: (v?: string) => v ? <Tag>{v}</Tag> : <Text type="secondary">-</Text> },
  ]

  const hasTasks = (ui.user_tasks || []).some(t => t.key || t.label || t.priority)
  const shared = ui.shared

  const hasLegacy = !surfaceKeys.length && (
    ui.list_intent?.trim() || ui.form_intent?.trim() || ui.detail_intent?.trim() ||
    ui.empty_state?.trim() || (ui.interaction_notes || []).length || (ui.surface_notes || []).length
  )

  const items: Array<{ label: string; children: React.ReactNode }> = [
    { label: '产品目标', children: ui.product_goal?.trim() || <Text type="secondary">-</Text> },
  ]

  if (hasTasks) {
    items.push({
      label: '跨端用户任务',
      children: (
        <Table
          columns={taskColumns}
          dataSource={ui.user_tasks || []}
          rowKey={(_, i) => String(i)}
          size="small"
          pagination={false}
        />
      ),
    })
  }

  if (shared) {
    items.push(
      { label: '共用实体', children: renderTagList(shared.primary_entities) },
      { label: '共用动作', children: renderTagList(shared.primary_actions) },
      { label: '共用空状态', children: shared.empty_state?.trim() || <Text type="secondary">-</Text> },
      { label: '统一术语', children: renderTerminology(shared.terminology) },
    )
  } else if (ui.empty_state?.trim()) {
    items.push({ label: '空状态', children: ui.empty_state })
  }

  return (
    <>
      <Descriptions
        bordered
        size="small"
        column={1}
        title="UI 意图"
        items={items}
      />

      {orderedKeys.length > 0 && (
        <Card size="small" title="端侧配置">
          <Collapse
            items={orderedKeys.map(surface => ({
              key: surface,
              label: `${SURFACE_LABELS[surface] || surface}${surfacesConfig[surface]?.role?.trim() ? ` · ${surfacesConfig[surface].role}` : ''}`,
              children: renderSurfacePanel(surface, surfacesConfig[surface] || {}),
            }))}
          />
        </Card>
      )}

      {hasLegacy && (
        <Descriptions
          bordered
          size="small"
          column={1}
          title="其他（旧配置）"
          items={[
            { label: '列表视图意图', children: ui.list_intent?.trim() || <Text type="secondary">-</Text> },
            { label: '表单视图意图', children: ui.form_intent?.trim() || <Text type="secondary">-</Text> },
            { label: '详情视图意图', children: ui.detail_intent?.trim() || <Text type="secondary">-</Text> },
            { label: '空状态', children: ui.empty_state?.trim() || <Text type="secondary">-</Text> },
            { label: '交互说明', children: renderTextList(ui.interaction_notes) },
            { label: '端侧说明', children: renderTextList(ui.surface_notes) },
          ]}
        />
      )}
    </>
  )
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
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailModel, setDetailModel] = useState<ModelIR | null>(null)
  const [historyOpen, setHistoryOpen] = useState(false)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [historyModel, setHistoryModel] = useState<ModelSummary | null>(null)
  const [historyVersions, setHistoryVersions] = useState<ModelVersionInfo[]>([])

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

  const handleViewDetail = async (modelSummary: ModelSummary & { _module: string }) => {
    setDetailOpen(true)
    setDetailLoading(true)
    setDetailModel(null)
    try {
      const res = await getModel(modelSummary._module, modelSummary.name)
      setDetailModel(res.data)
    } catch (err: any) {
      message.error(err?.response?.data?.error || err?.message || '加载模型详情失败')
    } finally {
      setDetailLoading(false)
    }
  }

  const handleViewHistory = async (modelSummary: ModelSummary & { _module: string }) => {
    setHistoryModel(modelSummary)
    setHistoryOpen(true)
    setHistoryLoading(true)
    setHistoryVersions([])
    try {
      const res = await getModelHistory(modelSummary._module, modelSummary.name)
      setHistoryVersions(res.data || [])
    } catch (err: any) {
      message.error(err?.response?.data?.error || err?.message || '加载修改历史失败')
    } finally {
      setHistoryLoading(false)
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
              icon={<EyeOutlined />}
              onClick={() => handleViewDetail(record)}
            >
              查看
            </Button>
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
            <Button
              type="link"
              size="small"
              icon={<HistoryOutlined />}
              onClick={() => handleViewHistory(record)}
            >
              历史
            </Button>
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
        title="模型主线由 AI agent 与人的对话维护"
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
          <Form.Item label="API 前缀">
            <Space.Compact style={{ width: '100%' }}>
              <Input style={{ width: 40 }} value="/" disabled />
              <Form.Item name="api_prefix" noStyle>
                <Input placeholder="content" />
              </Form.Item>
            </Space.Compact>
          </Form.Item>
          <Form.Item name="icon" label="图标">
            <IconSelect placeholder="选择模块图标，如 AppstoreOutlined" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="模块用途说明" />
          </Form.Item>
        </Form>
      </Modal>

      <ModelDetailDrawer
        open={detailOpen}
        loading={detailLoading}
        model={detailModel}
        onClose={() => setDetailOpen(false)}
        onEdit={(model) => navigate(buildRoute(`/modules/${model.module}/models/${model.name}`))}
      />

      <ModelHistoryModal
        open={historyOpen}
        loading={historyLoading}
        model={historyModel}
        versions={historyVersions}
        onCancel={() => setHistoryOpen(false)}
      />
    </div>
  )
}

interface ModelDetailDrawerProps {
  open: boolean
  loading: boolean
  model: ModelIR | null
  onClose: () => void
  onEdit: (model: ModelIR) => void
}

const ModelDetailDrawer: React.FC<ModelDetailDrawerProps> = ({ open, loading, model, onClose, onEdit }) => {
  const fieldColumns: ColumnsType<FieldIR> = [
    { title: '字段', dataIndex: 'name', key: 'name', width: 140, render: (v: string) => <Text code>{v}</Text> },
    { title: '标签', dataIndex: 'label', key: 'label', width: 120 },
    { title: '类型', dataIndex: 'type', key: 'type', width: 100, render: (v: string) => <Tag>{v}</Tag> },
    { title: '必填', dataIndex: 'required', key: 'required', width: 70, render: renderBool },
    { title: '唯一', dataIndex: 'unique', key: 'unique', width: 70, render: renderBool },
    { title: '默认值', dataIndex: 'default', key: 'default', render: (v?: string) => v || <Text type="secondary">-</Text> },
  ]

  const relationColumns: ColumnsType<RelationIR> = [
    { title: '关联名', dataIndex: 'name', key: 'name', width: 150, render: (v: string) => <Text code>{v}</Text> },
    { title: '关系谓词', dataIndex: 'label', key: 'label', width: 120, render: (v?: string) => v || <Text type="secondary">-</Text> },
    { title: '关系类型', dataIndex: 'type', key: 'type', width: 130, render: (v: string) => <Tag>{v}</Tag> },
    { title: '目标', dataIndex: 'target', key: 'target', render: (v: string, r) => v || r.target_model || <Text type="secondary">-</Text> },
    { title: '反向导航属性', dataIndex: 'back_ref', key: 'back_ref', width: 150, render: (v?: string) => v || <Text type="secondary">-</Text> },
  ]

  return (
    <Drawer
      title={model ? `查看模型 ${model.module}/${model.name}` : '查看模型'}
      open={open}
      onClose={onClose}
      size={920}
      loading={loading}
      extra={
        model ? (
          <Button type="primary" icon={<EditOutlined />} onClick={() => onEdit(model)}>
            编辑
          </Button>
        ) : null
      }
    >
      {model ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Descriptions
            bordered
            size="small"
            column={2}
            items={[
              { label: '模块', children: <Tag>{model.module}</Tag> },
              { label: '模型名', children: <Text code>{model.name}</Text> },
              { label: '显示名', children: model.label || <Text type="secondary">-</Text> },
              { label: '图标', children: model.display?.icon ? <Tag>{model.display.icon}</Tag> : <Text type="secondary">-</Text> },
              { label: '表名', children: <Text code>{model.table}</Text> },
              { label: '描述', span: 2, children: model.description || <Text type="secondary">-</Text> },
              { label: '列表字段', span: 2, children: renderTagList(model.admin?.list_columns) },
              { label: '搜索字段', span: 2, children: renderTagList(model.admin?.search_fields) },
              { label: '默认排序', children: model.admin?.default_sort || <Text type="secondary">-</Text> },
              { label: '排序方向', children: model.admin?.default_order || <Text type="secondary">-</Text> },
            ]}
          />

          <Card size="small" title={`字段（${model.fields?.length || 0}）`}>
            <Table columns={fieldColumns} dataSource={model.fields || []} rowKey="name" size="small" pagination={false} />
          </Card>

          <Card size="small" title={`关联（${model.relations?.length || 0}）`}>
            <Table columns={relationColumns} dataSource={model.relations || []} rowKey="name" size="small" pagination={false} />
          </Card>

          <Descriptions
            bordered
            size="small"
            column={1}
            title="业务语义"
            items={[
              { label: '用途', children: model.semantics?.purpose || <Text type="secondary">-</Text> },
              { label: '用户群体', children: renderTextList(model.semantics?.audience) },
              { label: '用户价值', children: model.semantics?.user_value || <Text type="secondary">-</Text> },
              { label: '业务规则', children: renderTextList(model.semantics?.business_rules) },
              { label: '权限', children: renderTextList(model.semantics?.permissions) },
              { label: '生命周期', children: renderTextList(model.semantics?.lifecycle) },
            ]}
          />

          {model.ui_intent && hasUIIntent(model.ui_intent) && renderUIIntent(model.ui_intent)}

          <Descriptions
            bordered
            size="small"
            column={1}
            title="API 意图"
            items={[
              { label: '开放范围', children: model.api_intent?.exposure || <Text type="secondary">-</Text> },
              { label: '调用方', children: renderTextList(model.api_intent?.consumers) },
              { label: '认证方式', children: model.api_intent?.auth || <Text type="secondary">-</Text> },
              { label: '基础路径', children: model.api_intent?.base_path ? <Text code>{model.api_intent.base_path}</Text> : <Text type="secondary">-</Text> },
              { label: '操作', children: renderTagList(model.api_intent?.operations) },
            ]}
          />
        </Space>
      ) : (
        <Empty description={loading ? '加载中' : '暂无模型详情'} />
      )}
    </Drawer>
  )
}

interface ModelHistoryModalProps {
  open: boolean
  loading: boolean
  model: ModelSummary | null
  versions: ModelVersionInfo[]
  onCancel: () => void
}

const ModelHistoryModal: React.FC<ModelHistoryModalProps> = ({ open, loading, model, versions, onCancel }) => {
  const columns: ColumnsType<ModelVersionInfo> = [
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 210,
      render: (v: string, record) => (
        <Space>
          <Tooltip title={v}>
            <Tag color={record.current ? 'blue' : undefined}>{formatVersionTime(v)}</Tag>
          </Tooltip>
          {record.current ? <Tag color="green">当前</Tag> : null}
        </Space>
      ),
    },
    {
      title: '本次变更',
      key: 'diff',
      width: 150,
      render: (_v, record) => <ChangeCountBadges summary={record.diff} />,
    },
    {
      title: '变更摘要',
      dataIndex: 'change_summary',
      key: 'change_summary',
      render: (v?: string) => v || <Text type="secondary">-</Text>,
    },
    {
      title: '修改人',
      dataIndex: 'created_by',
      key: 'created_by',
      width: 120,
      render: (v?: string) => v || <Text type="secondary">-</Text>,
    },
    {
      title: '修改时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: formatDateTime,
    },
  ]

  return (
    <Modal
      title={model ? `修改历史：${model.module}/${model.name}` : '修改历史'}
      open={open}
      onCancel={onCancel}
      footer={null}
      width={960}
    >
      <Table
        columns={columns}
        dataSource={versions}
        loading={loading}
        rowKey="version"
        size="small"
        pagination={versions.length > 8 ? { pageSize: 8 } : false}
        locale={{ emptyText: '暂无修改历史' }}
        expandable={{
          expandedRowRender: (record) => <ChangeList changes={record.diff?.changes} />,
          rowExpandable: (record) => !!record.diff?.has_changes,
        }}
      />
    </Modal>
  )
}

export default ModelOverview
