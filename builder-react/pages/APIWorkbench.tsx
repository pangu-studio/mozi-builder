import React, { useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  Descriptions,
  Empty,
  Form,
  Input,
  Modal,
  Radio,
  Select,
  Space,
  Spin,
  Table,
  Tabs,
  Tag,
  Typography,
  Tooltip,
  message,
  Card,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { ApiOutlined, EditOutlined, ReloadOutlined, SendOutlined } from '@ant-design/icons'
import {
  getMoziBuilderApiClient,
  listModels,
  listAPIAssets,
  saveAPIEndpointOverride,
  type APIAssetIndex,
  type APIEndpointAsset,
  type APIModuleSummary,
  type ModuleSummary,
  type APISurface,
} from '../api/dev-platform'

const { Title, Text, Paragraph } = Typography

const SURFACE_META: Record<APISurface, { label: string; color: string }> = {
  admin: { label: '管理后台', color: 'purple' },
  miniapp: { label: '小程序', color: 'cyan' },
  desktop: { label: '桌面端', color: 'geekblue' },
  'client-shared': { label: '客户端共用', color: 'green' },
  internal: { label: '内部', color: 'orange' },
  public: { label: '公开', color: 'blue' },
}

const METHOD_COLORS: Record<string, string> = {
  GET: 'green',
  POST: 'blue',
  PUT: 'orange',
  PATCH: 'gold',
  DELETE: 'red',
}

const APIWorkbench: React.FC = () => {
  const [assetIndex, setAssetIndex] = useState<APIAssetIndex | null>(null)
  const [loading, setLoading] = useState(false)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [surface, setSurface] = useState<APISurface | undefined>()
  const [moduleName, setModuleName] = useState<string | undefined>()
  const [keyword, setKeyword] = useState('')
  const [debugPathParams, setDebugPathParams] = useState<Record<string, string>>({})
  const [debugParams, setDebugParams] = useState('{}')
  const [debugBody, setDebugBody] = useState('{}')
  const [debugLoading, setDebugLoading] = useState(false)
  const [debugResult, setDebugResult] = useState<string>('')
  const [moduleOptions, setModuleOptions] = useState<APIModuleSummary[]>([])
  const [moduleLoading, setModuleLoading] = useState(false)
  const [authMode, setAuthMode] = useState<'current' | 'custom'>('current')
  const [customToken, setCustomToken] = useState('')
  const [assignOpen, setAssignOpen] = useState(false)
  const [assigningEndpoint, setAssigningEndpoint] = useState<APIEndpointAsset | null>(null)

  const load = async () => {
    setLoading(true)
    try {
      const res = await listAPIAssets()
      setAssetIndex(res.data)
      await loadModuleOptions(res.data.modules)
      if (!selectedId && res.data.endpoints.length > 0) {
        setSelectedId(res.data.endpoints[0].id)
      }
    } catch (err: any) {
      message.error(err?.response?.data?.error || err?.message || '加载 API 资产失败')
    } finally {
      setLoading(false)
    }
  }

  const loadModuleOptions = async (baseModules: APIModuleSummary[] = assetIndex?.modules || []) => {
    setModuleLoading(true)
    try {
      const res = await listModels()
      setModuleOptions(mergeModuleOptions(baseModules, res.data))
    } catch (err: any) {
      setModuleOptions(baseModules)
      message.warning(err?.response?.data?.error || err?.message || '加载模块列表失败，已使用 API 资产模块')
    } finally {
      setModuleLoading(false)
    }
  }

  const openAssignModule = (endpoint: APIEndpointAsset) => {
    setAssigningEndpoint(endpoint)
    setAssignOpen(true)
  }

  useEffect(() => {
    load()
  }, [])

  const endpoints = assetIndex?.endpoints || []
  const filteredEndpoints = useMemo(() => {
    const kw = keyword.trim().toLowerCase()
    return endpoints.filter((endpoint) => {
      if (surface && endpoint.surface !== surface) return false
      if (moduleName && endpoint.module !== moduleName) return false
      if (!kw) return true
      return [
        endpoint.method,
        endpoint.path,
        endpoint.summary,
        endpoint.description,
        endpoint.operation_id,
        endpoint.module,
        ...(endpoint.tags || []),
        ...(endpoint.business_models || []),
      ]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(kw))
    })
  }, [endpoints, surface, moduleName, keyword])

  const selectedEndpoint = endpoints.find((endpoint) => endpoint.id === selectedId) || filteredEndpoints[0]

  useEffect(() => {
    if (selectedEndpoint) {
      setDebugPathParams(buildDefaultPathParams(selectedEndpoint))
      setDebugParams(buildDefaultParams(selectedEndpoint))
      setDebugBody(buildDefaultBody(selectedEndpoint))
    }
  }, [selectedEndpoint?.id])

  const columns: ColumnsType<APIEndpointAsset> = [
    {
      title: '方法',
      dataIndex: 'method',
      key: 'method',
      width: 82,
      render: (value: string) => <Tag color={METHOD_COLORS[value] || 'default'}>{value}</Tag>,
    },
    {
      title: '路径',
      dataIndex: 'path',
      key: 'path',
      ellipsis: true,
      render: (value: string, record) => (
        <div>
          <Text code>{value}</Text>
          <div style={{ marginTop: 4 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {record.display_name || record.summary || record.operation_id || '未命名接口'}
            </Text>
          </div>
        </div>
      ),
    },
    {
      title: '端',
      dataIndex: 'surface',
      key: 'surface',
      width: 110,
      render: (value: APISurface) => {
        const meta = SURFACE_META[value] || { label: value, color: 'default' }
        return <Tag color={meta.color}>{meta.label}</Tag>
      },
    },
    {
      title: '模块',
      dataIndex: 'module',
      key: 'module',
      width: 120,
      render: (_: string, record) =>
        record.module ? <Tag>{record.module_label || record.module}</Tag> : <Text type="secondary">未关联</Text>,
    },
    {
      title: '操作',
      key: 'actions',
      width: 80,
      render: (_: unknown, record) => (
        <Tooltip title={record.module ? '修改模块' : '分配模块'}>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={(e) => {
              e.stopPropagation()
              openAssignModule(record)
            }}
          />
        </Tooltip>
      ),
    },
  ]

  const handleDebug = async () => {
    if (!selectedEndpoint) return
    setDebugLoading(true)
    setDebugResult('')
    try {
      const params = parseJSON(debugParams, 'Query 参数')
      const data = selectedEndpoint.method === 'GET' || selectedEndpoint.method === 'DELETE'
        ? undefined
        : parseJSON(debugBody, '请求 Body')
      const headers: Record<string, string> = {}
      if (authMode === 'custom' && customToken.trim()) {
        headers.Authorization = `Bearer ${customToken.trim()}`
      }
      // Substitute path parameters into the URL
      const filledPath = fillPathParams(selectedEndpoint.path, debugPathParams)
      const res = await getMoziBuilderApiClient().request({
        method: selectedEndpoint.method,
        url: toAxiosURL(filledPath),
        params,
        data,
        headers: Object.keys(headers).length > 0 ? headers : undefined,
      })
      setDebugResult(JSON.stringify({ status: res.status, data: res.data }, null, 2))
    } catch (err: any) {
      const payload = err?.response
        ? { status: err.response.status, data: err.response.data }
        : { error: err?.message || String(err) }
      setDebugResult(JSON.stringify(payload, null, 2))
    } finally {
      setDebugLoading(false)
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>
            API Workbench
          </Title>
          <Text type="secondary">从 OpenAPI 提取接口、Schema、端类型和模型关联，用于查看与调试。</Text>
        </div>
        <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
          刷新
        </Button>
      </div>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        title="API 设计与实施由 Agent 辅助完成"
        description="这里展示代码导出的真实 OpenAPI 契约。显示名、模块归属、示例参数等可以后续做轻量微调；HTTP 方法、URL、请求响应结构应回到 Agent 和代码层修改。"
      />

      {loading && !assetIndex ? (
        <div style={{ textAlign: 'center', padding: 80 }}>
          <Spin size="large" description="正在解析 OpenAPI..." />
        </div>
      ) : !assetIndex ? (
        <Empty description="暂无 API 资产数据" />
      ) : (
        <>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: 12, marginBottom: 16 }}>
            <Metric title="接口数" value={assetIndex.summary.endpoint_count} />
            <Metric title="Schema 数" value={assetIndex.summary.schema_count} />
            <Metric title="模块数" value={assetIndex.summary.module_count} />
            <Metric title="端类型" value={assetIndex.summary.surface_count} />
          </div>

          <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <Space style={{ marginBottom: 12 }} wrap>
                <Select
                  allowClear
                  placeholder="全部端类型"
                  style={{ width: 160 }}
                  value={surface}
                  onChange={setSurface}
                  options={assetIndex.surfaces.map((item) => ({
                    value: item.surface,
                    label: `${SURFACE_META[item.surface]?.label || item.surface} (${item.endpoint_count})`,
                  }))}
                />
                <Select
                  allowClear
                  placeholder="全部模块"
                  style={{ width: 180 }}
                  value={moduleName}
                  onChange={setModuleName}
                  loading={moduleLoading}
                  onDropdownVisibleChange={(open) => {
                    if (open) loadModuleOptions(assetIndex.modules)
                  }}
                  options={moduleOptions.map((item) => ({
                    value: item.name,
                    label: `${item.label || item.name} (${item.endpoint_count})`,
                  }))}
                />
                <Input.Search
                  allowClear
                  placeholder="搜索路径、摘要、模型"
                  style={{ width: 280 }}
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                />
              </Space>
              <Table
                rowKey="id"
                columns={columns}
                dataSource={filteredEndpoints}
                size="middle"
                pagination={{ pageSize: 10, showSizeChanger: false }}
                onRow={(record) => ({
                  onClick: () => setSelectedId(record.id),
                })}
                rowClassName={(record) => (record.id === selectedEndpoint?.id ? 'ant-table-row-selected' : '')}
              />
            </div>

            <div style={{ width: 440, flexShrink: 0 }}>
              {selectedEndpoint ? (
                <Card
                  size="small"
                  title={
                    <Space>
                      <ApiOutlined />
                      <span>接口详情</span>
                    </Space>
                  }
                >
                  <Tabs
                    size="small"
                    items={[
                      {
                        key: 'detail',
                        label: '契约',
                        children: (
                          <EndpointDetail endpoint={selectedEndpoint} />
                        ),
                      },
                      {
                        key: 'debug',
                        label: '调试',
                        children: (
                          <div>
                            <Form layout="vertical" size="small">
                              <Form.Item label="认证方式">
                                <Radio.Group
                                  value={authMode}
                                  onChange={(e) => setAuthMode(e.target.value)}
                                >
                                  <Radio.Button value="current">当前账号</Radio.Button>
                                  <Radio.Button value="custom">自定义 Token</Radio.Button>
                                </Radio.Group>
                                {authMode === 'custom' && (
                                  <Input.Password
                                    style={{ marginTop: 8 }}
                                    placeholder="输入 Bearer Token"
                                    value={customToken}
                                    onChange={(e) => setCustomToken(e.target.value)}
                                  />
                                )}
                              </Form.Item>
                              {Object.keys(debugPathParams).length > 0 && (
                                <Form.Item label="Path 参数">
                                  <Space direction="vertical" style={{ width: '100%' }}>
                                    {(selectedEndpoint?.parameters || [])
                                      .filter((p) => p.in === 'path')
                                      .map((p) => (
                                        <Space.Compact key={p.name} style={{ width: '100%' }}>
                                          <Input style={{ width: 140 }} value={p.name} disabled />
                                          <Input
                                            placeholder={p.description || `输入 ${p.name}`}
                                            value={debugPathParams[p.name] || ''}
                                            onChange={(e) =>
                                              setDebugPathParams((prev) => ({
                                                ...prev,
                                                [p.name]: e.target.value,
                                              }))
                                            }
                                          />
                                        </Space.Compact>
                                      ))}
                                  </Space>
                                </Form.Item>
                              )}
                              <Form.Item label="Query 参数 JSON">
                                <Input.TextArea
                                  rows={4}
                                  value={debugParams}
                                  onChange={(event) => setDebugParams(event.target.value)}
                                />
                              </Form.Item>
                              <Form.Item label="请求 Body JSON">
                                <Input.TextArea
                                  rows={6}
                                  value={debugBody}
                                  disabled={selectedEndpoint.method === 'GET' || selectedEndpoint.method === 'DELETE'}
                                  onChange={(event) => setDebugBody(event.target.value)}
                                />
                              </Form.Item>
                            </Form>
                            <Button
                              type="primary"
                              icon={<SendOutlined />}
                              loading={debugLoading}
                              onClick={handleDebug}
                            >
                              发送请求
                            </Button>
                            {debugResult && (
                              <pre
                                style={{
                                  marginTop: 12,
                                  padding: 12,
                                  background: '#f6f8fa',
                                  borderRadius: 6,
                                  maxHeight: 300,
                                  overflow: 'auto',
                                }}
                              >
                                {debugResult}
                              </pre>
                            )}
                          </div>
                        ),
                      },
                    ]}
                  />
                </Card>
              ) : (
                <Empty description="选择一个接口查看详情" />
              )}
            </div>
          </div>
        </>
      )}

      <ModuleAssignModal
        open={assignOpen}
        endpoint={assigningEndpoint}
        modules={moduleOptions}
        loadingModules={moduleLoading}
        onOpenModules={() => loadModuleOptions(assetIndex?.modules || [])}
        onCancel={() => setAssignOpen(false)}
        onSaved={load}
      />
    </div>
  )
}

const Metric: React.FC<{ title: string; value: number }> = ({ title, value }) => (
  <div style={{ border: '1px solid #f0f0f0', borderRadius: 8, padding: 16 }}>
    <Text type="secondary">{title}</Text>
    <div style={{ fontSize: 26, fontWeight: 600, marginTop: 4 }}>{value}</div>
  </div>
)

const EndpointDetail: React.FC<{
  endpoint: APIEndpointAsset
}> = ({ endpoint }) => {
  const surfaceMeta = SURFACE_META[endpoint.surface] || { label: endpoint.surface, color: 'default' }

  return (
    <div>
      <Descriptions
        size="small"
        column={1}
        items={[
          {
            label: '方法',
            children: <Tag color={METHOD_COLORS[endpoint.method] || 'default'}>{endpoint.method}</Tag>,
          },
          { label: '路径', children: <Text code>{endpoint.path}</Text> },
          { label: '端类型', children: <Tag color={surfaceMeta.color}>{surfaceMeta.label}</Tag> },
          {
            label: '模块',
            children: endpoint.module ? (
              <Space>
                <Tag color={endpoint.module_overridden ? 'blue' : undefined}>
                  {endpoint.module_label || endpoint.module}
                </Tag>
                {endpoint.module_overridden ? <Text type="secondary">后台关联</Text> : null}
              </Space>
            ) : (
              <Text type="secondary">未关联</Text>
            ),
          },
          { label: '认证', children: endpoint.auth_required ? <Tag color="red">需要</Tag> : <Tag>不需要</Tag> },
          { label: 'operationId', children: endpoint.operation_id || '-' },
        ]}
      />
      <Paragraph style={{ marginTop: 12 }}>
        <Text strong>说明：</Text>
        <br />
        {endpoint.description || endpoint.display_name || endpoint.summary || '暂无说明'}
      </Paragraph>
      <TagList title="业务模型" values={endpoint.business_models} />
      <TagList title="请求 Schema" values={endpoint.request_schemas} />
      <TagList title="响应 Schema" values={endpoint.response_schemas} />
      <TagList title="Tags" values={endpoint.tags} />
      <div style={{ marginTop: 12 }}>
        <Text type="secondary">契约 Hash：</Text>
        <Text code>{endpoint.source_hash.slice(0, 12)}</Text>
      </div>
    </div>
  )
}

const ModuleAssignModal: React.FC<{
  open: boolean
  endpoint: APIEndpointAsset | null
  modules: APIModuleSummary[]
  loadingModules: boolean
  onOpenModules: () => void
  onCancel: () => void
  onSaved: () => Promise<void>
}> = ({ open, endpoint, modules, loadingModules, onOpenModules, onCancel, onSaved }) => {
  const [moduleID, setModuleID] = useState<string | undefined>(undefined)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (open && endpoint) {
      setModuleID(endpoint.module || undefined)
    }
  }, [open, endpoint?.id, endpoint?.module])

  const handleSave = async () => {
    if (!endpoint) return
    setSaving(true)
    try {
      await saveAPIEndpointOverride({
        endpoint_id: endpoint.id,
        module_id: moduleID || '',
      })
      message.success('已保存接口模块关联')
      await onSaved()
      onCancel()
    } catch (err: any) {
      message.error(err?.response?.data?.error || err?.message || '保存模块关联失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      title={endpoint?.module ? '修改模块关联' : '分配模块'}
      open={open}
      onOk={handleSave}
      onCancel={onCancel}
      confirmLoading={saving}
      okText="保存"
      cancelText="取消"
    >
      {endpoint && (
        <div>
          <div style={{ marginBottom: 12 }}>
            <Tag color={METHOD_COLORS[endpoint.method] || 'default'}>{endpoint.method}</Tag>
            <Text code style={{ marginLeft: 4 }}>
              {endpoint.path}
            </Text>
            <div style={{ marginTop: 4 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {endpoint.display_name || endpoint.summary || endpoint.operation_id || '未命名接口'}
              </Text>
            </div>
          </div>
          {endpoint.module ? (
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                当前模块：
              </Text>
              <Tag color={endpoint.module_overridden ? 'blue' : undefined}>
                {endpoint.module_label || endpoint.module}
              </Tag>
              {endpoint.module_overridden ? (
                <Text type="secondary" style={{ fontSize: 12 }}>
                  后台关联
                </Text>
              ) : null}
            </div>
          ) : null}
          <Form layout="vertical">
            <Form.Item label="所属模块" style={{ marginBottom: 8 }}>
              <Select
                allowClear
                placeholder="选择模块（留空表示不关联）"
                style={{ width: '100%' }}
                value={moduleID}
                onChange={setModuleID}
                loading={loadingModules}
                onDropdownVisibleChange={(visible) => {
                  if (visible) onOpenModules()
                }}
                options={modules.map((item) => ({
                  value: item.name,
                  label: `${item.label || item.name} (${item.name})`,
                }))}
              />
            </Form.Item>
          </Form>
          <Text type="secondary" style={{ fontSize: 12 }}>
            只保存平台展示关联，不修改 OpenAPI 路径、方法或请求响应结构。
          </Text>
        </div>
      )}
    </Modal>
  )
}

const TagList: React.FC<{ title: string; values: string[] }> = ({ title, values }) => (
  <div style={{ marginTop: 12 }}>
    <Text strong>{title}</Text>
    <div style={{ marginTop: 6 }}>
      {values && values.length > 0 ? values.map((value) => <Tag key={value}>{value}</Tag>) : <Text type="secondary">无</Text>}
    </div>
  </div>
)

function buildDefaultPathParams(endpoint: APIEndpointAsset): Record<string, string> {
  const pathParams = (endpoint.parameters || []).filter((p) => p.in === 'path')
  const obj: Record<string, string> = {}
  for (const p of pathParams) {
    obj[p.name] = ''
  }
  return obj
}

function fillPathParams(path: string, params: Record<string, string>): string {
  let result = path
  for (const [name, value] of Object.entries(params)) {
    if (!value) continue
    result = result.replace(`{${name}}`, encodeURIComponent(value))
    result = result.replace(`:${name}`, encodeURIComponent(value))
  }
  return result
}

function buildDefaultParams(endpoint: APIEndpointAsset): string {
  const queryParams = (endpoint.parameters || []).filter((p) => p.in === 'query')
  if (queryParams.length === 0) return '{}'
  const obj: Record<string, any> = {}
  for (const p of queryParams) {
    obj[p.name] = defaultValueForType(p.type)
  }
  return JSON.stringify(obj, null, 2)
}

function buildDefaultBody(endpoint: APIEndpointAsset): string {
  if (endpoint.method === 'GET' || endpoint.method === 'DELETE') return '{}'
  if (endpoint.request_body_example) {
    return JSON.stringify(endpoint.request_body_example, null, 2)
  }
  // Fallback: build from body parameters
  const bodyParams = (endpoint.parameters || []).filter((p) => p.in === 'body')
  if (bodyParams.length === 0) return '{}'
  const obj: Record<string, any> = {}
  for (const p of bodyParams) {
    obj[p.name] = defaultValueForType(p.type)
  }
  return JSON.stringify(obj, null, 2)
}

function defaultValueForType(type: string): any {
  switch (type) {
    case 'integer':
    case 'int':
      return 0
    case 'number':
    case 'float':
      return 0.0
    case 'boolean':
    case 'bool':
      return false
    case 'array':
      return []
    case 'object':
      return {}
    case 'string':
    default:
      return ''
  }
}

function parseJSON(value: string, label: string) {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  try {
    return JSON.parse(trimmed)
  } catch {
    throw new Error(`${label} 不是合法 JSON`)
  }
}

function toAxiosURL(path: string) {
  if (/^https?:\/\//.test(path)) return path
  if (path === '/api') return '/'
  if (path.startsWith('/api/')) return path.slice(4)
  return `${window.location.origin}${path}`
}

function mergeModuleOptions(apiModules: APIModuleSummary[], modelModules: ModuleSummary[]) {
  const merged = new Map<string, APIModuleSummary>()
  for (const item of apiModules || []) {
    merged.set(item.name, item)
  }
  for (const item of modelModules || []) {
    const existing = merged.get(item.name)
    merged.set(item.name, {
      name: item.name,
      label: item.label || existing?.label || item.name,
      endpoint_count: existing?.endpoint_count || 0,
      model_count: item.model_count || existing?.model_count || 0,
    })
  }
  return Array.from(merged.values()).sort((a, b) => a.name.localeCompare(b.name))
}

export default APIWorkbench
