import React, { useEffect, useState, useCallback } from 'react'
import {
  Typography,
  Button,
  Space,
  Input,
  Card,
  Tag,
  message,
  Breadcrumb,
  Spin,
  Select,
  Switch,
  InputNumber,
  Table,
  Collapse,
  Drawer,
  Alert,
  Modal,
  Popconfirm,
} from 'antd'
import {
  ArrowLeftOutlined,
  SaveOutlined,
  CheckCircleOutlined,
  DiffOutlined,
  PlusOutlined,
  CodeOutlined,
} from '@ant-design/icons'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import FieldTable from '../components/FieldTable'
import FieldEditor from '../components/FieldEditor'
import RelationEditor from '../components/RelationEditor'
import ModelYamlPreview from '../components/ModelYamlPreview'
import { useDevPlatformStore } from '../stores/dev-platform'
import { useMoziBuilder } from '..'
import {
  API_AUTH_OPTIONS,
  API_EXPOSURE_OPTIONS,
  API_OPERATION_OPTIONS,
  deleteDesignDictionaryItem,
  listDesignDictionaryItems,
  saveDesignDictionaryItem,
  type DesignDictionaryItem,
  type FieldIR,
  type RelationIR,
  type ModelIR,
} from '../api/dev-platform'

const { Title, Text } = Typography
const { TextArea } = Input
const API_CONSUMERS_DICTIONARY = 'api_consumers'

const defaultField: FieldIR = {
  name: '',
  type: 'string',
  label: '',
  form_type: 'text',
}

const defaultRelation: RelationIR = {
  name: '',
  type: 'has_many',
  target: '',
}

function normalizeDictionaryValues(values: string[], items: DesignDictionaryItem[]) {
  const aliasMap = new Map<string, string>()
  for (const item of items) {
    const candidates = [item.value, item.label, ...(item.aliases || [])]
    for (const candidate of candidates) {
      const normalized = candidate?.trim().toLowerCase()
      if (normalized) {
        aliasMap.set(normalized, item.value)
      }
    }
  }

  const result: string[] = []
  for (const value of values) {
    const trimmed = value?.trim()
    if (!trimmed) continue
    const normalized = aliasMap.get(trimmed.toLowerCase()) || trimmed
    if (!result.includes(normalized)) {
      result.push(normalized)
    }
  }
  return result
}

const ModelDesigner: React.FC = () => {
  const navigate = useNavigate()
  const { buildRoute } = useMoziBuilder()
  const location = useLocation()
  const { module, name } = useParams<{ module?: string; name?: string }>()
  const isNew = !module && name === 'new'
  const initialModule = (location.state as { module?: string } | null)?.module || 'content'

  const {
    modules,
    currentModel,
    modelLoading,
    error,
    loadModel,
    loadModules,
    saveModel,
    createNewModel,
    validateModelAction,
    validateResult,
    clearError,
    resetCurrentModel,
  } = useDevPlatformStore()

  // 本地编辑状态
  const [localModel, setLocalModel] = useState<ModelIR | null>(null)
  const [saving, setSaving] = useState(false)
  const [validating, setValidating] = useState(false)

  // 弹窗状态
  const [fieldEditorVisible, setFieldEditorVisible] = useState(false)
  const [editingField, setEditingField] = useState<FieldIR | null>(null)
  const [editingFieldIndex, setEditingFieldIndex] = useState<number | null>(null)
  const [relationEditorVisible, setRelationEditorVisible] = useState(false)
  const [editingRelation, setEditingRelation] = useState<RelationIR | null>(null)
  const [editingRelationIndex, setEditingRelationIndex] = useState<number | null>(null)
  const [yamlDrawerVisible, setYamlDrawerVisible] = useState(false)
  const [apiConsumerItems, setAPIConsumerItems] = useState<DesignDictionaryItem[]>([])
  const [apiConsumerLoading, setAPIConsumerLoading] = useState(false)
  const [apiConsumerManagerOpen, setAPIConsumerManagerOpen] = useState(false)

  useEffect(() => {
    loadModules()
    if (!isNew && module && name) {
      loadModel(module, name)
    } else {
      resetCurrentModel()
      setLocalModel(null)
    }
    return () => {
      resetCurrentModel()
    }
  }, [module, name, isNew])

  useEffect(() => {
    if (currentModel && !isNew) {
      setLocalModel({
        ...currentModel,
        fields: [...(currentModel.fields || [])],
        relations: [...(currentModel.relations || [])],
        semantics: { ...(currentModel.semantics || {}) },
        ui_intent: { ...(currentModel.ui_intent || {}) },
        api_intent: { ...(currentModel.api_intent || {}) },
      })
    }
  }, [currentModel, isNew])

  useEffect(() => {
    if (error) {
      message.error(error.split('\n')[0])
      clearError()
    }
  }, [error])

  const loadAPIConsumerItems = useCallback(async (includeDisabled = false) => {
    setAPIConsumerLoading(true)
    try {
      const res = await listDesignDictionaryItems(API_CONSUMERS_DICTIONARY, includeDisabled)
      setAPIConsumerItems(res.data || [])
    } catch (err: any) {
      message.warning(err?.response?.data?.error || err?.message || '加载 API 调用方字典失败')
    } finally {
      setAPIConsumerLoading(false)
    }
  }, [])

  useEffect(() => {
    loadAPIConsumerItems(false)
  }, [loadAPIConsumerItems])

  useEffect(() => {
    if (!localModel || apiConsumerItems.length === 0) return
    const currentConsumers = localModel.api_intent?.consumers || []
    const normalizedConsumers = normalizeDictionaryValues(currentConsumers, apiConsumerItems)
    if (normalizedConsumers.join('\n') !== currentConsumers.join('\n')) {
      setLocalModel({
        ...localModel,
        api_intent: { ...(localModel.api_intent || {}), consumers: normalizedConsumers },
      })
    }
  }, [apiConsumerItems, localModel])

  // 初始化新模型
  useEffect(() => {
    if (isNew) {
      setLocalModel({
        module: initialModule,
        name: '',
        label: '',
        description: '',
        table: '',
        fields: [],
        relations: [],
        semantics: {},
        ui_intent: {},
        api_intent: {},
        admin: {
          list_columns: [],
          search_fields: [],
          default_sort: 'created_at',
          default_order: 'desc',
          page_size: 20,
        },
      })
    }
  }, [isNew, initialModule])

  const handleSave = async () => {
    if (!localModel) return
    if (!localModel.name.trim()) {
      message.error('请输入模型名称')
      return
    }
    setSaving(true)
    try {
      if (isNew) {
        await createNewModel(localModel)
        message.success('模型创建成功')
        navigate(buildRoute(`/modules/${localModel.module}/models/${localModel.name}`))
      } else {
        await saveModel(module!, name!, localModel)
        message.success('模型已保存，新版本已生成')
      }
    } catch {
      message.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleValidate = async () => {
    if (!localModel || isNew) return
    setValidating(true)
    try {
      await validateModelAction(module!, name!)
      // 结果在 validateResult 中
    } catch {
      message.error('校验失败')
    } finally {
      setValidating(false)
    }
  }

  // 字段操作
  const handleAddField = () => {
    setEditingField(null)
    setEditingFieldIndex(null)
    setFieldEditorVisible(true)
  }

  const handleEditField = (field: FieldIR, index: number) => {
    setEditingField(field)
    setEditingFieldIndex(index)
    setFieldEditorVisible(true)
  }

  const handleDeleteField = (_field: FieldIR, index: number) => {
    if (!localModel) return
    const newFields = localModel.fields.filter((_, i) => i !== index)
    setLocalModel({ ...localModel, fields: newFields })
  }

  const handleFieldOk = (field: FieldIR) => {
    if (!localModel) return
    const newFields = [...localModel.fields]
    if (editingFieldIndex !== null && editingFieldIndex >= 0) {
      newFields[editingFieldIndex] = field
    } else {
      newFields.push(field)
    }
    setLocalModel({ ...localModel, fields: newFields })
    setFieldEditorVisible(false)
  }

  const handleMoveField = (from: number, to: number) => {
    if (!localModel) return
    const newFields = [...localModel.fields]
    const [moved] = newFields.splice(from, 1)
    newFields.splice(to, 0, moved)
    setLocalModel({ ...localModel, fields: newFields })
  }

  // 关联操作
  const handleAddRelation = () => {
    setEditingRelation(null)
    setEditingRelationIndex(null)
    setRelationEditorVisible(true)
  }

  const handleEditRelation = (relation: RelationIR, index: number) => {
    setEditingRelation(relation)
    setEditingRelationIndex(index)
    setRelationEditorVisible(true)
  }

  const handleDeleteRelation = (_relation: RelationIR, index: number) => {
    if (!localModel) return
    const newRelations = localModel.relations.filter((_, i) => i !== index)
    setLocalModel({ ...localModel, relations: newRelations })
  }

  const handleRelationOk = (relation: RelationIR) => {
    if (!localModel) return
    const newRelations = [...localModel.relations]
    if (editingRelationIndex !== null && editingRelationIndex >= 0) {
      newRelations[editingRelationIndex] = relation
    } else {
      newRelations.push(relation)
    }
    setLocalModel({ ...localModel, relations: newRelations })
    setRelationEditorVisible(false)
  }

  // 后台配置变更
  const handleAdminChange = (key: string, value: unknown) => {
    if (!localModel) return
    setLocalModel({
      ...localModel,
      admin: { ...localModel.admin, [key]: value },
    })
  }

  const handleSemanticsChange = (key: string, value: unknown) => {
    if (!localModel) return
    setLocalModel({
      ...localModel,
      semantics: { ...(localModel.semantics || {}), [key]: value },
    })
  }

  const handleUIIntentChange = (key: string, value: unknown) => {
    if (!localModel) return
    setLocalModel({
      ...localModel,
      ui_intent: { ...(localModel.ui_intent || {}), [key]: value },
    })
  }

  const handleUISharedChange = (key: string, value: unknown) => {
    if (!localModel) return
    setLocalModel({
      ...localModel,
      ui_intent: {
        ...(localModel.ui_intent || {}),
        shared: { ...(localModel.ui_intent?.shared || {}), [key]: value },
      },
    })
  }

  const handleUISurfaceChange = (surface: string, key: string, value: unknown) => {
    if (!localModel) return
    const currentSurfaces = localModel.ui_intent?.surfaces_config || {}
    setLocalModel({
      ...localModel,
      ui_intent: {
        ...(localModel.ui_intent || {}),
        surfaces_config: {
          ...currentSurfaces,
          [surface]: { ...(currentSurfaces[surface] || {}), [key]: value },
        },
      },
    })
  }

  const handleUISurfaceViewChange = (surface: string, view: string, key: string, value: unknown) => {
    if (!localModel) return
    const currentSurfaces = localModel.ui_intent?.surfaces_config || {}
    const currentSurface = currentSurfaces[surface] || {}
    const currentViews = currentSurface.views || {}
    setLocalModel({
      ...localModel,
      ui_intent: {
        ...(localModel.ui_intent || {}),
        surfaces_config: {
          ...currentSurfaces,
          [surface]: {
            ...currentSurface,
            views: {
              ...currentViews,
              [view]: { ...(currentViews[view] || {}), [key]: value },
            },
          },
        },
      },
    })
  }

  const handleAPIIntentChange = (key: string, value: unknown) => {
    if (!localModel) return
    const nextValue = key === 'consumers' && Array.isArray(value)
      ? normalizeDictionaryValues(value, apiConsumerItems)
      : value
    setLocalModel({
      ...localModel,
      api_intent: { ...(localModel.api_intent || {}), [key]: nextValue },
    })
  }

  const apiConsumerOptions = (() => {
    const selected = localModel?.api_intent?.consumers || []
    const optionMap = new Map<string, { value: string; label: string }>()
    apiConsumerItems
      .filter((item) => item.enabled !== false)
      .forEach((item) => optionMap.set(item.value, { value: item.value, label: item.label || item.value }))
    selected.forEach((value) => {
      if (!optionMap.has(value)) {
        optionMap.set(value, { value, label: value })
      }
    })
    return Array.from(optionMap.values())
  })()

  const updateAPIConsumerDraft = (index: number, patch: Partial<DesignDictionaryItem>) => {
    setAPIConsumerItems((items) => items.map((item, i) => (i === index ? { ...item, ...patch } : item)))
  }

  const handleAddAPIConsumer = () => {
    setAPIConsumerItems((items) => [
      ...items,
      {
        dictionary_id: API_CONSUMERS_DICTIONARY,
        value: '',
        label: '',
        aliases: [],
        sort_order: (items.length + 1) * 10,
        enabled: true,
      },
    ])
  }

  const handleSaveAPIConsumer = async (item: DesignDictionaryItem) => {
    const value = item.value.trim()
    if (!value) {
      message.error('请输入保存值')
      return
    }
    await saveDesignDictionaryItem(API_CONSUMERS_DICTIONARY, {
      ...item,
      value,
      label: item.label?.trim() || value,
      aliases: (item.aliases || []).map((alias) => alias.trim()).filter(Boolean),
      enabled: item.enabled !== false,
    })
    message.success('调用方选项已保存')
    await loadAPIConsumerItems(true)
  }

  const handleDeleteAPIConsumer = async (value: string) => {
    if (!value.trim()) {
      setAPIConsumerItems((items) => items.filter((item) => item.value.trim()))
      return
    }
    await deleteDesignDictionaryItem(API_CONSUMERS_DICTIONARY, value)
    message.success('调用方选项已删除')
    await loadAPIConsumerItems(true)
  }

  const linesToText = (items?: string[]) => (items || []).join('\n')
  const textToLines = (text: string) => text.split('\n').map((line) => line.trim()).filter(Boolean)
  const tasksToText = (items?: { key?: string; label?: string; priority?: string }[]) =>
    (items || []).map((item) => [item.key, item.label, item.priority].filter(Boolean).join(' | ')).join('\n')
  const textToTasks = (text: string) =>
    textToLines(text).map((line) => {
      const [key = '', label = '', priority = ''] = line.split('|').map((part) => part.trim())
      return { key, label, priority }
    })
  const terminologyToText = (items?: Record<string, string>) =>
    Object.entries(items || {}).map(([key, value]) => `${key}: ${value}`).join('\n')
  const textToTerminology = (text: string) =>
    Object.fromEntries(
      textToLines(text).map((line) => {
        const [key, ...rest] = line.split(':')
        return [key.trim(), rest.join(':').trim()]
      }).filter(([key]) => key),
    )

  // 展平所有模型供关联选择
  const allModelSummaries = modules.flatMap((mod) =>
    (mod.models || []).map((m) => ({ ...m, module: mod.name })),
  )

  const renderSurfaceIntent = (surface: string, placeholder: string) => {
    const config = localModel?.ui_intent?.surfaces_config?.[surface] || {}
    const listView = config.views?.list || {}
    const detailView = config.views?.detail || {}
    const formView = config.views?.form || {}

    return (
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '12px 24px' }}>
        <div style={{ flex: '1 1 320px', minWidth: 260 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>端侧角色</Text>
          <Input
            placeholder={placeholder}
            value={config.role || ''}
            onChange={(e) => handleUISurfaceChange(surface, 'role', e.target.value)}
          />
        </div>
        <div style={{ flex: '1 1 320px', minWidth: 260 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>启用任务</Text>
          <Select
            mode="tags"
            style={{ width: '100%' }}
            placeholder="如 find_deck、review_deck"
            value={config.enabled_tasks || []}
            onChange={(v) => handleUISurfaceChange(surface, 'enabled_tasks', v)}
          />
        </div>
        <div style={{ flex: '1 1 320px', minWidth: 260 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>端侧动作</Text>
          <Select
            mode="tags"
            style={{ width: '100%' }}
            placeholder="如 batch_archive、quick_edit、review"
            value={config.actions || []}
            onChange={(v) => handleUISurfaceChange(surface, 'actions', v)}
          />
        </div>
        <div style={{ flex: '1 1 420px', minWidth: 280 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>列表视图意图</Text>
          <TextArea
            rows={3}
            placeholder="列表在这个端主要帮助用户判断什么、采取什么动作"
            value={listView.intent || ''}
            onChange={(e) => handleUISurfaceViewChange(surface, 'list', 'intent', e.target.value)}
          />
        </div>
        <div style={{ flex: '1 1 220px', minWidth: 180 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>列表密度</Text>
          <Select
            allowClear
            style={{ width: '100%' }}
            value={listView.density}
            onChange={(v) => handleUISurfaceViewChange(surface, 'list', 'density', v)}
            options={[
              { value: 'high', label: '高密度' },
              { value: 'medium', label: '中密度' },
              { value: 'low', label: '低密度' },
            ]}
          />
        </div>
        <div style={{ flex: '1 1 320px', minWidth: 260 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>列表字段</Text>
          <Select
            mode="tags"
            style={{ width: '100%' }}
            value={listView.fields || []}
            onChange={(v) => handleUISurfaceViewChange(surface, 'list', 'fields', v)}
          />
        </div>
        <div style={{ flex: '1 1 420px', minWidth: 280 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>详情视图意图</Text>
          <TextArea
            rows={3}
            placeholder="详情页在这个端应突出哪些信息和后续动作"
            value={detailView.intent || ''}
            onChange={(e) => handleUISurfaceViewChange(surface, 'detail', 'intent', e.target.value)}
          />
        </div>
        <div style={{ flex: '1 1 420px', minWidth: 280 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>表单视图意图</Text>
          <TextArea
            rows={3}
            placeholder="创建/编辑在这个端应该如何分组、校验、精简或增强"
            value={formView.intent || ''}
            onChange={(e) => handleUISurfaceViewChange(surface, 'form', 'intent', e.target.value)}
          />
        </div>
        <div style={{ flex: '1 1 100%', minWidth: 280 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>端侧约束（一行一条）</Text>
          <TextArea
            rows={4}
            placeholder="如 支持快捷键；优先单手操作；保留批量筛选"
            value={linesToText(config.constraints)}
            onChange={(e) => handleUISurfaceChange(surface, 'constraints', textToLines(e.target.value))}
          />
        </div>
      </div>
    )
  }

  const apiConsumerColumns = [
    {
      title: '保存值',
      dataIndex: 'value',
      width: 160,
      render: (_: unknown, record: DesignDictionaryItem, index: number) => (
        <Input
          placeholder="如 mobile_app"
          value={record.value}
          onChange={(e) => updateAPIConsumerDraft(index, { value: e.target.value })}
        />
      ),
    },
    {
      title: '展示名',
      dataIndex: 'label',
      width: 180,
      render: (_: unknown, record: DesignDictionaryItem, index: number) => (
        <Input
          placeholder="如 移动 App"
          value={record.label}
          onChange={(e) => updateAPIConsumerDraft(index, { label: e.target.value })}
        />
      ),
    },
    {
      title: '别名',
      dataIndex: 'aliases',
      render: (_: unknown, record: DesignDictionaryItem, index: number) => (
        <Select
          mode="tags"
          style={{ width: '100%' }}
          placeholder="兼容历史写法"
          value={record.aliases || []}
          onChange={(aliases) => updateAPIConsumerDraft(index, { aliases })}
        />
      ),
    },
    {
      title: '排序',
      dataIndex: 'sort_order',
      width: 100,
      render: (_: unknown, record: DesignDictionaryItem, index: number) => (
        <InputNumber
          style={{ width: '100%' }}
          value={record.sort_order || 0}
          onChange={(sortOrder) => updateAPIConsumerDraft(index, { sort_order: sortOrder || 0 })}
        />
      ),
    },
    {
      title: '启用',
      dataIndex: 'enabled',
      width: 80,
      render: (_: unknown, record: DesignDictionaryItem, index: number) => (
        <Switch
          checked={record.enabled !== false}
          onChange={(enabled) => updateAPIConsumerDraft(index, { enabled })}
        />
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      render: (_: unknown, record: DesignDictionaryItem) => (
        <Space size={4}>
          <Button type="link" size="small" onClick={() => handleSaveAPIConsumer(record)}>
            保存
          </Button>
          <Popconfirm
            title="删除调用方选项？"
            onConfirm={() => handleDeleteAPIConsumer(record.value)}
          >
            <Button type="link" danger size="small">
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  if (modelLoading) {
    return (
      <div style={{ textAlign: 'center', padding: 80 }}>
        <Spin size="large" tip="正在加载模型..." />
      </div>
    )
  }

  const modelName = isNew ? '补录模型' : name

  return (
    <div>
      {/* 顶部 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Space>
          <Button
            icon={<ArrowLeftOutlined />}
            onClick={() => navigate(buildRoute('/models'))}
          >
            返回
          </Button>
          <Breadcrumb
            items={[
              { title: '开发平台', onClick: () => navigate(buildRoute('/models')) },
              { title: isNew ? '补录模型' : `模型微调：${currentModel?.label || name}（${currentModel?.name || ''}）` },
            ]}
          />
        </Space>
        <Space>
          {!isNew && (
            <>
              <Button
                icon={<CheckCircleOutlined />}
                onClick={handleValidate}
                loading={validating}
              >
                校验
              </Button>
              {validateResult && (
                <Space size={4}>
                  {validateResult.valid ? (
                    <Tag color="green">校验通过</Tag>
                  ) : (
                    <Tag color="red">{validateResult.errors.length} 个错误</Tag>
                  )}
                  {validateResult.warnings.length > 0 && (
                    <Tag color="orange">{validateResult.warnings.length} 个警告</Tag>
                  )}
                </Space>
              )}
              <Button
                icon={<DiffOutlined />}
                onClick={() => navigate(buildRoute(`/modules/${module}/models/${name}/diff`))}
              >
                AI 变更计划
              </Button>
            </>
          )}
          <Button
            icon={<CodeOutlined />}
            onClick={() => setYamlDrawerVisible(true)}
          >
            YAML
          </Button>
          <Button
            type="primary"
            icon={<SaveOutlined />}
            onClick={handleSave}
            loading={saving}
          >
            保存
          </Button>
        </Space>
      </div>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="这里更适合查看和微调模型"
        description="完整建模建议先在 AI agent 会话中完成，再回到这里检查字段、关系、业务语义、UI 意图和 API 意图是否符合预期。"
      />

      {/* 校验结果详情 */}
      {validateResult && (!validateResult.valid || validateResult.warnings.length > 0) && (
        <Card size="small" style={{ marginBottom: 16 }}>
          {validateResult.errors.map((e, i) => (
            <div key={`err-${i}`} style={{ color: '#ff4d4f', fontSize: 13, marginBottom: 4 }}>
              ❌ {e}
            </div>
          ))}
          {validateResult.warnings.map((w, i) => (
            <div key={`warn-${i}`} style={{ color: '#faad14', fontSize: 13, marginBottom: 4 }}>
              ⚠️ {w}
            </div>
          ))}
        </Card>
      )}

      {/* 顶部折叠面板：基础信息 + 后台展示配置 */}
      {localModel && (
        <Collapse
          defaultActiveKey={['basic']}
          style={{ marginBottom: 16 }}
          items={[
            {
              key: 'basic',
              label: '基础信息',
              children: (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '12px 24px' }}>
                  <div style={{ flex: '1 1 200px', minWidth: 180 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>所属模块</Text>
                    <Select
                      style={{ width: '100%' }}
                      value={localModel.module}
                      onChange={(v) => setLocalModel({ ...localModel, module: v })}
                      options={modules.map((m) => ({ value: m.name, label: `${m.label} (${m.name})` }))}
                    />
                  </div>
                  <div style={{ flex: '1 1 200px', minWidth: 180 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>模型名称 *</Text>
                    <Input
                      placeholder="如 User, Deck"
                      value={localModel.name}
                      onChange={(e) => setLocalModel({ ...localModel, name: e.target.value })}
                      disabled={!isNew}
                    />
                  </div>
                  <div style={{ flex: '1 1 200px', minWidth: 180 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>中文标签 *</Text>
                    <Input
                      placeholder="如 用户"
                      value={localModel.label}
                      onChange={(e) => setLocalModel({ ...localModel, label: e.target.value })}
                    />
                  </div>
                  <div style={{ flex: '1 1 200px', minWidth: 180 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>数据库表名</Text>
                    <Input
                      placeholder="如 users"
                      value={localModel.table}
                      onChange={(e) => setLocalModel({ ...localModel, table: e.target.value })}
                    />
                  </div>
                  <div style={{ flex: '1 1 200px', minWidth: 180 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>图标</Text>
                    <Input
                      placeholder="如 UserOutlined"
                      value={localModel.display?.icon || ''}
                      onChange={(e) =>
                        setLocalModel({ ...localModel, display: { ...localModel.display, icon: e.target.value } })
                      }
                    />
                  </div>
                  <div style={{ flex: '2 1 400px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>描述</Text>
                    <TextArea
                      rows={2}
                      placeholder="模型描述"
                      value={localModel.description}
                      onChange={(e) => setLocalModel({ ...localModel, description: e.target.value })}
                    />
                  </div>
                </div>
              ),
            },
            {
              key: 'semantics',
              label: '业务语义',
              children: (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '12px 24px' }}>
                  <div style={{ flex: '2 1 420px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>业务目的</Text>
                    <TextArea
                      rows={2}
                      placeholder="这个模型在业务里代表什么，为什么存在"
                      value={localModel.semantics?.purpose || ''}
                      onChange={(e) => handleSemanticsChange('purpose', e.target.value)}
                    />
                  </div>
                  <div style={{ flex: '2 1 420px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>用户价值</Text>
                    <TextArea
                      rows={2}
                      placeholder="它给用户或运营带来的价值"
                      value={localModel.semantics?.user_value || ''}
                      onChange={(e) => handleSemanticsChange('user_value', e.target.value)}
                    />
                  </div>
                  <div style={{ flex: '1 1 280px', minWidth: 240 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>使用人群</Text>
                    <Select
                      mode="tags"
                      style={{ width: '100%' }}
                      placeholder="如 普通用户、管理员、运营"
                      value={localModel.semantics?.audience || []}
                      onChange={(v) => handleSemanticsChange('audience', v)}
                    />
                  </div>
                  <div style={{ flex: '1 1 280px', minWidth: 240 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>权限语义</Text>
                    <Select
                      mode="tags"
                      style={{ width: '100%' }}
                      placeholder="如 仅创建者可编辑、管理员可删除"
                      value={localModel.semantics?.permissions || []}
                      onChange={(v) => handleSemanticsChange('permissions', v)}
                    />
                  </div>
                  <div style={{ flex: '1 1 360px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>业务规则（一行一条）</Text>
                    <TextArea
                      rows={4}
                      placeholder="如 已发布内容不可直接删除"
                      value={linesToText(localModel.semantics?.business_rules)}
                      onChange={(e) => handleSemanticsChange('business_rules', textToLines(e.target.value))}
                    />
                  </div>
                  <div style={{ flex: '1 1 360px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>生命周期（一行一条）</Text>
                    <TextArea
                      rows={4}
                      placeholder="如 draft -> published -> archived"
                      value={linesToText(localModel.semantics?.lifecycle)}
                      onChange={(e) => handleSemanticsChange('lifecycle', textToLines(e.target.value))}
                    />
                  </div>
                </div>
              ),
            },
            {
              key: 'ui_intent',
              label: 'UI 意图',
              children: (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '12px 24px' }}>
                  <div style={{ flex: '1 1 100%', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>产品目标</Text>
                    <TextArea
                      rows={2}
                      placeholder="这个模型在各端共同帮助用户完成什么"
                      value={localModel.ui_intent?.product_goal || ''}
                      onChange={(e) => handleUIIntentChange('product_goal', e.target.value)}
                    />
                  </div>
                  <div style={{ flex: '1 1 420px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>跨端用户任务（一行一条：key | 名称 | 优先级）</Text>
                    <TextArea
                      rows={4}
                      placeholder="如 find_deck | 找到要复习的牌组 | high"
                      value={tasksToText(localModel.ui_intent?.user_tasks)}
                      onChange={(e) => handleUIIntentChange('user_tasks', textToTasks(e.target.value))}
                    />
                  </div>
                  <div style={{ flex: '1 1 320px', minWidth: 260 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>共用实体</Text>
                    <Select
                      mode="tags"
                      style={{ width: '100%' }}
                      placeholder="如 deck、card、review"
                      value={localModel.ui_intent?.shared?.primary_entities || []}
                      onChange={(v) => handleUISharedChange('primary_entities', v)}
                    />
                  </div>
                  <div style={{ flex: '1 1 320px', minWidth: 260 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>共用动作</Text>
                    <Select
                      mode="tags"
                      style={{ width: '100%' }}
                      placeholder="如 create、edit、archive、review"
                      value={localModel.ui_intent?.shared?.primary_actions || []}
                      onChange={(v) => handleUISharedChange('primary_actions', v)}
                    />
                  </div>
                  <div style={{ flex: '1 1 360px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>共用空状态</Text>
                    <TextArea
                      rows={3}
                      placeholder="各端无数据时共同表达什么"
                      value={localModel.ui_intent?.shared?.empty_state || ''}
                      onChange={(e) => handleUISharedChange('empty_state', e.target.value)}
                    />
                  </div>
                  <div style={{ flex: '1 1 360px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>统一术语（一行一条：key: 展示名）</Text>
                    <TextArea
                      rows={3}
                      placeholder="如 deck: 牌组"
                      value={terminologyToText(localModel.ui_intent?.shared?.terminology)}
                      onChange={(e) => handleUISharedChange('terminology', textToTerminology(e.target.value))}
                    />
                  </div>
                  <div style={{ flex: '1 1 100%', minWidth: 280 }}>
                    <Collapse
                      items={[
                        { key: 'admin', label: '管理后台', children: renderSurfaceIntent('admin', '高密度管理、运营检索、批量操作') },
                        { key: 'desktop', label: '桌面客户端', children: renderSurfaceIntent('desktop', '离线、本地数据、快捷键、复习入口') },
                        { key: 'miniapp', label: '小程序', children: renderSurfaceIntent('miniapp', '轻量路径、单手操作、弱网和移动端限制') },
                      ]}
                    />
                  </div>
                </div>
              ),
            },
            {
              key: 'admin',
              label: '后台展示配置',
              children: (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '12px 24px' }}>
                  <div style={{ flex: '1 1 280px', minWidth: 240 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>列表展示列</Text>
                    <Select
                      mode="multiple"
                      style={{ width: '100%' }}
                      placeholder="选择列表展示的字段"
                      value={localModel.admin.list_columns || []}
                      onChange={(v) => handleAdminChange('list_columns', v)}
                      options={localModel.fields.map((f) => ({ value: f.name, label: f.label || f.name }))}
                    />
                  </div>
                  <div style={{ flex: '1 1 280px', minWidth: 240 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>搜索字段</Text>
                    <Select
                      mode="multiple"
                      style={{ width: '100%' }}
                      placeholder="选择可搜索的字段"
                      value={localModel.admin.search_fields || []}
                      onChange={(v) => handleAdminChange('search_fields', v)}
                      options={localModel.fields
                        .filter((f) => f.searchable)
                        .map((f) => ({ value: f.name, label: f.label || f.name }))}
                    />
                  </div>
                  <div style={{ flex: '1 1 160px', minWidth: 140 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>默认排序字段</Text>
                    <Input
                      value={localModel.admin.default_sort}
                      onChange={(e) => handleAdminChange('default_sort', e.target.value)}
                    />
                  </div>
                  <div style={{ flex: '1 1 120px', minWidth: 100 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>排序方向</Text>
                    <Select
                      style={{ width: '100%' }}
                      value={localModel.admin.default_order}
                      onChange={(v) => handleAdminChange('default_order', v)}
                      options={[
                        { value: 'desc', label: '降序' },
                        { value: 'asc', label: '升序' },
                      ]}
                    />
                  </div>
                  <div style={{ flex: '1 1 120px', minWidth: 100 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>每页条数</Text>
                    <InputNumber
                      style={{ width: '100%' }}
                      min={5}
                      max={100}
                      value={localModel.admin.page_size}
                      onChange={(v) => handleAdminChange('page_size', v || 20)}
                    />
                  </div>
                </div>
              ),
            },
            {
              key: 'api_intent',
              label: 'API 意图',
              children: (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '12px 24px' }}>
                  <div style={{ flex: '1 1 220px', minWidth: 180 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>开放范围</Text>
                    <Select
                      style={{ width: '100%' }}
                      placeholder="选择 API 使用范围"
                      allowClear
                      value={localModel.api_intent?.exposure}
                      onChange={(v) => handleAPIIntentChange('exposure', v)}
                      options={API_EXPOSURE_OPTIONS}
                    />
                  </div>
                  <div style={{ flex: '1 1 260px', minWidth: 220 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>认证方式</Text>
                    <Select
                      showSearch
                      allowClear
                      style={{ width: '100%' }}
                      placeholder="选择认证方式"
                      value={localModel.api_intent?.auth || undefined}
                      onChange={(v) => handleAPIIntentChange('auth', v)}
                      options={API_AUTH_OPTIONS}
                    />
                  </div>
                  <div style={{ flex: '1 1 300px', minWidth: 240 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>基础路径</Text>
                    <Input
                      placeholder={`如 /api/${localModel.module}/${localModel.table || localModel.name}`}
                      value={localModel.api_intent?.base_path || ''}
                      onChange={(e) => handleAPIIntentChange('base_path', e.target.value)}
                    />
                  </div>
                  <div style={{ flex: '2 1 360px', minWidth: 260 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <Text type="secondary" style={{ fontSize: 12 }}>调用方</Text>
                      <Button
                        type="link"
                        size="small"
                        style={{ padding: 0, height: 20 }}
                        onClick={() => {
                          setAPIConsumerManagerOpen(true)
                          loadAPIConsumerItems(true)
                        }}
                      >
                        维护
                      </Button>
                    </div>
                    <Select
                      mode="multiple"
                      showSearch
                      loading={apiConsumerLoading}
                      style={{ width: '100%' }}
                      placeholder="选择调用方"
                      value={localModel.api_intent?.consumers || []}
                      onChange={(v) => handleAPIIntentChange('consumers', v)}
                      options={apiConsumerOptions}
                    />
                  </div>
                  <div style={{ flex: '2 1 420px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>操作集合</Text>
                    <Select
                      mode="multiple"
                      showSearch
                      style={{ width: '100%' }}
                      placeholder="选择操作集合"
                      value={localModel.api_intent?.operations || []}
                      onChange={(v) => handleAPIIntentChange('operations', v)}
                      options={API_OPERATION_OPTIONS}
                    />
                  </div>
                  <div style={{ flex: '1 1 360px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>请求约束（一行一条）</Text>
                    <TextArea
                      rows={4}
                      placeholder="如 create 必须包含 deck_id；批量接口限制 100 条"
                      value={linesToText(localModel.api_intent?.request_notes)}
                      onChange={(e) => handleAPIIntentChange('request_notes', textToLines(e.target.value))}
                    />
                  </div>
                  <div style={{ flex: '1 1 360px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>响应约定（一行一条）</Text>
                    <TextArea
                      rows={4}
                      placeholder="如 列表返回 items/total；时间字段使用 RFC3339"
                      value={linesToText(localModel.api_intent?.response_notes)}
                      onChange={(e) => handleAPIIntentChange('response_notes', textToLines(e.target.value))}
                    />
                  </div>
                  <div style={{ flex: '1 1 360px', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>错误场景（一行一条）</Text>
                    <TextArea
                      rows={4}
                      placeholder="如 无权限返回 403；资源不存在返回 404"
                      value={linesToText(localModel.api_intent?.error_cases)}
                      onChange={(e) => handleAPIIntentChange('error_cases', textToLines(e.target.value))}
                    />
                  </div>
                  <div style={{ flex: '1 1 280px', minWidth: 240 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>幂等策略</Text>
                    <Input
                      placeholder="如 upsert by client_id；POST 非幂等"
                      value={localModel.api_intent?.idempotency || ''}
                      onChange={(e) => handleAPIIntentChange('idempotency', e.target.value)}
                    />
                  </div>
                  <div style={{ flex: '1 1 280px', minWidth: 240 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>限流策略</Text>
                    <Input
                      placeholder="如 每用户每分钟 60 次"
                      value={localModel.api_intent?.rate_limit || ''}
                      onChange={(e) => handleAPIIntentChange('rate_limit', e.target.value)}
                    />
                  </div>
                  <div style={{ flex: '1 1 280px', minWidth: 240 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>版本策略</Text>
                    <Input
                      placeholder="如 /api/v1；新增字段保持向后兼容"
                      value={localModel.api_intent?.versioning || ''}
                      onChange={(e) => handleAPIIntentChange('versioning', e.target.value)}
                    />
                  </div>
                  <div style={{ flex: '1 1 100%', minWidth: 280 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>兼容性说明（一行一条）</Text>
                    <TextArea
                      rows={4}
                      placeholder="如 不删除已有响应字段；新增枚举值需客户端容错"
                      value={linesToText(localModel.api_intent?.compatibility_notes)}
                      onChange={(e) => handleAPIIntentChange('compatibility_notes', textToLines(e.target.value))}
                    />
                  </div>
                </div>
              ),
            },
          ]}
        />
      )}

      {/* 编辑区：字段 + 关联（全宽） */}
      <Card size="small" style={{ marginBottom: 16 }}>
        <FieldTable
          fields={localModel?.fields || []}
          loading={modelLoading}
          onAdd={handleAddField}
          onEdit={handleEditField}
          onDelete={handleDeleteField}
          onMove={handleMoveField}
        />
      </Card>

      <Card
        size="small"
        title="关联关系"
        extra={
          <Button size="small" type="dashed" icon={<PlusOutlined />} onClick={handleAddRelation}>
            添加关联
          </Button>
        }
      >
        {localModel?.relations && localModel.relations.length > 0 ? (
          <Table
            dataSource={localModel.relations.map((r, i) => ({ ...r, _index: i, key: r.name || `rel-${i}` }))}
            size="small"
            pagination={false}
            columns={[
              {
                title: '关联名',
                dataIndex: 'name',
                key: 'name',
                width: 100,
                render: (v: string) => <Text code>{v}</Text>,
              },
              {
                title: '类型',
                dataIndex: 'type',
                key: 'type',
                width: 100,
                render: (v: string) => <Tag>{v}</Tag>,
              },
              {
                title: '目标',
                key: 'target',
                render: (_: unknown, r: RelationIR & { _index: number }) => (
                  <Text code>{r.target_module}/{r.target_model}</Text>
                ),
              },
              {
                title: '操作',
                key: 'actions',
                width: 100,
                render: (_: unknown, r: RelationIR & { _index: number }) => (
                  <Space size={4}>
                    <Button
                      type="link"
                      size="small"
                      onClick={() => handleEditRelation(r, r._index)}
                    >
                      编辑
                    </Button>
                    <Button
                      type="link"
                      size="small"
                      danger
                      onClick={() => handleDeleteRelation(r, r._index)}
                    >
                      删除
                    </Button>
                  </Space>
                ),
              },
            ]}
          />
        ) : (
          <div style={{ padding: 24, textAlign: 'center', color: '#999', fontSize: 13 }}>
            暂无关联关系
          </div>
        )}
      </Card>

      {/* YAML 预览抽屉 */}
      <Drawer
        title="YAML 实时预览"
        placement="right"
        size="large"
        open={yamlDrawerVisible}
        onClose={() => setYamlDrawerVisible(false)}
      >
        <ModelYamlPreview model={localModel} />
      </Drawer>

      <Modal
        title="维护 API 调用方"
        open={apiConsumerManagerOpen}
        width={920}
        onCancel={() => {
          setAPIConsumerManagerOpen(false)
          loadAPIConsumerItems(false)
        }}
        footer={[
          <Button key="add" icon={<PlusOutlined />} onClick={handleAddAPIConsumer}>
            新增调用方
          </Button>,
          <Button
            key="close"
            type="primary"
            onClick={() => {
              setAPIConsumerManagerOpen(false)
              loadAPIConsumerItems(false)
            }}
          >
            完成
          </Button>,
        ]}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="调用方保存在当前业务的设计数据库中"
          description="保存值会写入模型 YAML；展示名只用于开发平台页面。别名用于兼容历史建模中的中文或旧命名。"
        />
        <Table
          rowKey={(record, index) => record.value || `new-${index}`}
          size="small"
          loading={apiConsumerLoading}
          pagination={false}
          dataSource={apiConsumerItems}
          columns={apiConsumerColumns}
        />
      </Modal>

      {/* 弹窗 */}
      <FieldEditor
        visible={fieldEditorVisible}
        field={editingField}
        onOk={handleFieldOk}
        onCancel={() => setFieldEditorVisible(false)}
      />

      <RelationEditor
        visible={relationEditorVisible}
        relation={editingRelation}
        models={allModelSummaries}
        currentModule={localModel?.module || 'content'}
        onOk={handleRelationOk}
        onCancel={() => setRelationEditorVisible(false)}
      />
    </div>
  )
}

export default ModelDesigner
