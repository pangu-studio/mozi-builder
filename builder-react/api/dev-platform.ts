import type { AxiosInstance } from 'axios'

// ====== Builder API client injection ======

export interface MoziBuilderApiConfig {
  client: AxiosInstance
  basePath?: string
}

let moziBuilderApiClient: AxiosInstance | null = null
let moziBuilderApiBasePath = '/dev-platform'

export function configureMoziBuilderApi(config: MoziBuilderApiConfig) {
  moziBuilderApiClient = config.client
  moziBuilderApiBasePath = normalizeBasePath(config.basePath || '/dev-platform')
}

export function getMoziBuilderApiClient() {
  if (!moziBuilderApiClient) {
    throw new Error('MoziBuilderProvider requires an apiClient')
  }
  return moziBuilderApiClient
}

export function getMoziBuilderApiBasePath() {
  return moziBuilderApiBasePath
}

function builderPath(path: string) {
  return `${moziBuilderApiBasePath}${path.startsWith('/') ? path : `/${path}`}`
}

function normalizeBasePath(basePath: string) {
  const trimmed = basePath.trim()
  if (!trimmed || trimmed === '/') return ''
  return trimmed.endsWith('/') ? trimmed.slice(0, -1) : trimmed
}

// ====== 类型定义 ======

export interface ModuleSummary {
  name: string
  label: string
  description?: string
  icon?: string
  api_prefix: string
  model_count?: number
  models?: ModelSummary[]
}

export interface ModelSummary {
  module: string
  name: string
  label: string
  description: string
  table: string
  field_count: number
  rel_count: number
  version: string
  sync_status: string // "synced" | "modified" | "new"
}

export interface ModelVersionInfo {
  version: string
  change_summary: string
  created_by: string
  created_at: string
  current: boolean
}

export interface FieldIR {
  name: string
  type: string // string | int | float | bool | time | text | enum | json
  label: string
  required?: boolean
  unique?: boolean
  sensitive?: boolean
  searchable?: boolean
  listable?: boolean
  editable?: boolean
  sortable?: boolean
  primary?: boolean
  auto_now_add?: boolean
  auto_now?: boolean
  default?: string
  enum_values?: string[]
  form_type?: string
  generated?: string
}

export interface RelationIR {
  name: string
  label?: string
  type: string // has_one | has_many | belongs_to | many_to_many
  target: string
  target_module?: string
  target_model?: string
  back_ref?: string
  cascade?: boolean
  required?: boolean
  unique?: boolean
}

export interface AdminConfig {
  list_columns: string[]
  search_fields: string[]
  default_sort: string
  default_order: string
  page_size: number
}

export interface SemanticConfig {
  purpose?: string
  audience?: string[]
  user_value?: string
  business_rules?: string[]
  permissions?: string[]
  lifecycle?: string[]
}

export interface UIIntentConfig {
  product_goal?: string
  user_tasks?: UIUserTaskConfig[]
  shared?: UISharedIntentConfig
  surfaces_config?: Record<string, UISurfaceIntentConfig>
  surfaces?: string[]
  primary_view?: string
  primary_actions?: string[]
  list_intent?: string
  form_intent?: string
  detail_intent?: string
  empty_state?: string
  interaction_notes?: string[]
  surface_notes?: string[]
}

export interface UIUserTaskConfig {
  key?: string
  label?: string
  priority?: string
}

export interface UISharedIntentConfig {
  primary_entities?: string[]
  primary_actions?: string[]
  empty_state?: string
  terminology?: Record<string, string>
}

export interface UISurfaceIntentConfig {
  role?: string
  enabled_tasks?: string[]
  views?: Record<string, UISurfaceViewConfig>
  actions?: string[]
  constraints?: string[]
}

export interface UISurfaceViewConfig {
  intent?: string
  density?: string
  fields?: string[]
}

export interface APIIntentConfig {
  exposure?: string
  consumers?: string[]
  auth?: string
  base_path?: string
  operations?: string[]
  request_notes?: string[]
  response_notes?: string[]
  error_cases?: string[]
  idempotency?: string
  rate_limit?: string
  versioning?: string
  compatibility_notes?: string[]
}

export const API_EXPOSURE_OPTIONS = [
  { value: 'none', label: '不开放' },
  { value: 'internal', label: '内部 API' },
  { value: 'client', label: '客户端 API' },
  { value: 'partner', label: '合作方 API' },
  { value: 'public', label: '公开 API' },
]

export const API_AUTH_OPTIONS = [
  { value: 'none', label: '无需认证' },
  { value: 'user_jwt', label: '用户 JWT' },
  { value: 'user_jwt_with_roles', label: '用户 JWT + 角色' },
  { value: 'admin_jwt', label: '管理后台 JWT' },
  { value: 'builder_jwt', label: 'Builder JWT' },
  { value: 'api_key', label: 'API Key' },
  { value: 'hmac', label: 'HMAC 签名' },
]

export const API_OPERATION_OPTIONS = [
  { value: 'list', label: 'list 列表' },
  { value: 'get', label: 'get 详情' },
  { value: 'create', label: 'create 创建' },
  { value: 'update', label: 'update 更新' },
  { value: 'delete', label: 'delete 删除' },
  { value: 'sync', label: 'sync 同步' },
  { value: 'review', label: 'review 复习' },
  { value: 'export', label: 'export 导出' },
  { value: 'import', label: 'import 导入' },
]

const API_EXPOSURE_ALIASES: Record<string, string> = {
  不开放: 'none',
  内部: 'internal',
  '内部 API': 'internal',
  客户端: 'client',
  '客户端 API': 'client',
  合作方: 'partner',
  '合作方 API': 'partner',
  公开: 'public',
  '公开 API': 'public',
}

const API_AUTH_ALIASES: Record<string, string> = {
  无: 'none',
  无需认证: 'none',
  public: 'none',
  JWT: 'user_jwt',
  jwt: 'user_jwt',
  用户JWT: 'user_jwt',
  '用户 JWT': 'user_jwt',
  admin_session: 'admin_jwt',
  admin_jwt_with_roles: 'admin_jwt',
  builder_jwt_with_roles: 'builder_jwt',
}

function normalizeChoice(value: string | undefined, aliases: Record<string, string>, allowedValues: string[]) {
  const trimmed = value?.trim()
  if (!trimmed) return undefined
  const lower = trimmed.toLowerCase()
  return aliases[trimmed] || aliases[lower] || (allowedValues.includes(lower) ? lower : trimmed)
}

function normalizeChoiceList(values: string[] | undefined, aliases: Record<string, string>, allowedValues: string[]) {
  const result: string[] = []
  for (const value of values || []) {
    const normalized = normalizeChoice(value, aliases, allowedValues)
    if (normalized && !result.includes(normalized)) {
      result.push(normalized)
    }
  }
  return result
}

function normalizeAPIIntent(apiIntent?: APIIntentConfig): APIIntentConfig {
  const exposureValues = API_EXPOSURE_OPTIONS.map((item) => item.value)
  const authValues = API_AUTH_OPTIONS.map((item) => item.value)
  const operationValues = API_OPERATION_OPTIONS.map((item) => item.value)

  return {
    ...(apiIntent || {}),
    exposure: normalizeChoice(apiIntent?.exposure, API_EXPOSURE_ALIASES, exposureValues),
    auth: normalizeChoice(apiIntent?.auth, API_AUTH_ALIASES, authValues),
    consumers: normalizeChoiceList(apiIntent?.consumers, {}, []),
    operations: normalizeChoiceList(apiIntent?.operations, {}, operationValues),
  }
}

export interface ModelIR {
  module: string
  name: string
  label: string
  description: string
  table: string
  fields: FieldIR[]
  relations: RelationIR[]
  admin: AdminConfig
  display?: { icon?: string }
  semantics?: SemanticConfig
  ui_intent?: UIIntentConfig
  api_intent?: APIIntentConfig
}

type BackendModelIR = Omit<ModelIR, 'name'> & {
  name?: string
  model?: string
}

export interface ValidateResult {
  valid: boolean
  errors: string[]
  warnings: string[]
}

export interface DiffChange {
  type: string // "added" | "removed" | "modified"
  category: string // "field" | "relation" | "admin"
  name: string
  detail: string
  old_value?: string
  new_value?: string
}

export interface AffectedFile {
  path: string
  description: string
  change_count: number
}

export interface DiffResult {
  model_ref: string
  from_version: string
  to_version: string
  changes: DiffChange[]
  has_changes: boolean
}

export interface ChangePlanTask {
  area: string
  description: string
  files?: string[]
}

export interface ChangePlanResult {
  model_ref: string
  status: 'pending' | 'applied' | 'no_diff'
  intent: string
  module_icon?: string
  model_icon?: string
  semantics: SemanticConfig
  ui_intent: UIIntentConfig
  api_intent: APIIntentConfig
  diff: DiffResult
  affected_files: AffectedFile[]
  contracts: string[]
  tasks: ChangePlanTask[]
  checks: string[]
  prompt: string
}

export type APISurface = 'admin' | 'miniapp' | 'desktop' | 'client-shared' | 'internal' | 'public'

export interface APIAssetSummary {
  endpoint_count: number
  schema_count: number
  module_count: number
  surface_count: number
}

export interface APIModuleSummary {
  name: string
  label: string
  endpoint_count: number
  model_count: number
}

export interface APISurfaceCount {
  surface: APISurface
  endpoint_count: number
}

export interface ParameterDetail {
  name: string
  in: string
  required: boolean
  type: string
  description?: string
}

export interface APIEndpointAsset {
  id: string
  method: string
  path: string
  operation_id?: string
  display_name?: string
  summary?: string
  description?: string
  tags: string[]
  surface: APISurface
  module?: string
  module_label?: string
  module_overridden: boolean
  business_models: string[]
  request_schemas: string[]
  response_schemas: string[]
  auth_required: boolean
  parameters: ParameterDetail[]
  request_body_example?: any
  status: string
  source_hash: string
}

export interface APISchemaAsset {
  name: string
  module?: string
  model?: string
  kind: string
  fields: string[]
  endpoint_ids: string[]
}

export interface APIAssetIndex {
  source: string
  title: string
  version: string
  base_path: string
  generated_by: string
  summary: APIAssetSummary
  modules: APIModuleSummary[]
  surfaces: APISurfaceCount[]
  endpoints: APIEndpointAsset[]
  schema_models: APISchemaAsset[]
}

export interface DesignDictionaryItem {
  dictionary_id: string
  value: string
  label: string
  description?: string
  aliases?: string[]
  sort_order?: number
  enabled?: boolean
  updated_at?: string
}

// ====== API 函数 ======

// 模型列表
export function listModels() {
  return getMoziBuilderApiClient().get<ModuleSummary[]>(builderPath('/models'))
}

// 创建模块
export function createModule(module: ModuleSummary) {
  return getMoziBuilderApiClient().post<ModuleSummary>(builderPath('/modules'), module)
}

// 更新模块元数据
export function updateModule(name: string, module: ModuleSummary) {
  return getMoziBuilderApiClient().put<ModuleSummary>(builderPath(`/modules/${name}`), module)
}

// 删除空模块
export function deleteModule(name: string) {
  return getMoziBuilderApiClient().delete(builderPath(`/modules/${name}`))
}

// 单个模型详情
export function getModel(module: string, name: string) {
  return getMoziBuilderApiClient().get<BackendModelIR>(builderPath(`/modules/${module}/models/${name}`)).then((res) => ({
    ...res,
    data: normalizeModel(res.data),
  }))
}

// 模型版本历史
export function getModelHistory(module: string, name: string) {
  return getMoziBuilderApiClient().get<ModelVersionInfo[]>(builderPath(`/modules/${module}/models/${name}/history`))
}

// 创建模型（发送结构化 JSON，后端负责标准化和持久化）
export function createModel(model: ModelIR) {
  return getMoziBuilderApiClient().post<BackendModelIR>(builderPath('/models'), toBackendModel(model)).then((res) => ({
    ...res,
    data: normalizeModel(res.data),
  }))
}

// 更新模型（发送结构化 JSON，避免前端手写 YAML 转义问题）
export function updateModel(module: string, name: string, model: ModelIR) {
  return getMoziBuilderApiClient().put<BackendModelIR>(builderPath(`/modules/${module}/models/${name}`), toBackendModel(model)).then((res) => ({
    ...res,
    data: normalizeModel(res.data),
  }))
}

// 删除模型
export function deleteModel(module: string, name: string) {
  return getMoziBuilderApiClient().delete(builderPath(`/modules/${module}/models/${name}`))
}

// ER 图 DSL（可选 module 参数筛选指定模块）
export function getERDiagram(module?: string) {
  return getMoziBuilderApiClient().get<string>(builderPath('/models/er'), {
    params: module ? { module } : undefined,
    responseType: 'text' as const,
  })
}

// 校验模型
export function validateModel(module: string, name: string) {
  return getMoziBuilderApiClient().post<ValidateResult>(builderPath(`/modules/${module}/models/${name}/validate`))
}

// 查看差异
export function getDiff(module: string, name: string) {
  return getMoziBuilderApiClient().get<DiffResult>(builderPath(`/modules/${module}/models/${name}/diff`))
}

// 获取 AI Coding 变更计划
export function getChangePlan(module: string, name: string) {
  return getMoziBuilderApiClient().get<ChangePlanResult>(builderPath(`/modules/${module}/models/${name}/change-plan`))
}

// 标记模型为已同步
export function syncModel(module: string, name: string) {
  return getMoziBuilderApiClient().post<{ status: string; model_ref: string }>(builderPath(`/modules/${module}/models/${name}/sync`))
}

// OpenAPI 资产索引
export function listAPIAssets() {
  return getMoziBuilderApiClient().get<APIAssetIndex>(builderPath('/apis'))
}

export function saveAPIEndpointOverride(data: {
  endpoint_id: string
  module_id?: string
  display_name?: string
  description?: string
}) {
  return getMoziBuilderApiClient().post<{ status: string; endpoint_id: string }>(builderPath('/apis/overrides'), data)
}

export function listDesignDictionaryItems(dictionary: string, includeDisabled = false) {
  return getMoziBuilderApiClient().get<DesignDictionaryItem[]>(builderPath(`/dictionaries/${dictionary}/items`), {
    params: includeDisabled ? { include_disabled: true } : undefined,
  })
}

export function saveDesignDictionaryItem(dictionary: string, item: DesignDictionaryItem) {
  return getMoziBuilderApiClient().post<{ status: string; dictionary_id: string; value: string }>(
    builderPath(`/dictionaries/${dictionary}/items`),
    item,
  )
}

export function deleteDesignDictionaryItem(dictionary: string, value: string) {
  return getMoziBuilderApiClient().delete<{ status: string; dictionary_id: string; value: string }>(
    builderPath(`/dictionaries/${dictionary}/items/${encodeURIComponent(value)}`),
  )
}

function normalizeModel(model: BackendModelIR): ModelIR {
  return {
    ...model,
    name: model.name || model.model || '',
    fields: model.fields || [],
    relations: model.relations || [],
    admin: model.admin || {
      list_columns: [],
      search_fields: [],
      default_sort: 'created_at',
      default_order: 'desc',
      page_size: 20,
    },
    semantics: model.semantics || {},
    ui_intent: model.ui_intent || {},
    api_intent: normalizeAPIIntent(model.api_intent),
  }
}

function toBackendModel(model: ModelIR): BackendModelIR {
  const { name, ...rest } = model
  return { ...rest, api_intent: normalizeAPIIntent(model.api_intent), model: name }
}
