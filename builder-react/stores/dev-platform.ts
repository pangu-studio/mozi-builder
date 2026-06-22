import { create } from 'zustand'
import {
  listModels,
  getModel,
  createModel,
  updateModel,
  deleteModel,
  createModule,
  updateModule,
  deleteModule,
  getERDiagram,
  validateModel,
  getDiff,
  getChangePlan,
  type ModuleSummary,
  type ModelIR,
  type DiffResult,
  type ChangePlanResult,
  type ValidateResult,
} from '../api/dev-platform'

interface DevPlatformState {
  // 数据
  modules: ModuleSummary[]
  erDsl: string
  currentModel: ModelIR | null
  diffResult: DiffResult | null
  changePlan: ChangePlanResult | null
  validateResult: ValidateResult | null

  // 加载状态
  loading: boolean
  erLoading: boolean
  modelLoading: boolean
  diffLoading: boolean
  changePlanLoading: boolean

  // 错误
  error: string | null

  // Actions
  loadModules: () => Promise<void>
  loadERDiagram: (module?: string) => Promise<void>
  loadModel: (module: string, name: string) => Promise<void>
  saveModel: (module: string, name: string, model: ModelIR) => Promise<ModelIR>
  createNewModel: (model: ModelIR) => Promise<ModelIR>
  removeModel: (module: string, name: string) => Promise<void>
  createNewModule: (module: ModuleSummary) => Promise<ModuleSummary>
  saveModule: (name: string, module: ModuleSummary) => Promise<ModuleSummary>
  removeModule: (name: string) => Promise<void>
  validateModelAction: (module: string, name: string) => Promise<void>
  loadDiff: (module: string, name: string) => Promise<void>
  loadChangePlan: (module: string, name: string) => Promise<void>
  clearError: () => void
  resetCurrentModel: () => void
}

export const useDevPlatformStore = create<DevPlatformState>((set, get) => ({
  // 初始状态
  modules: [],
  erDsl: '',
  currentModel: null,
  diffResult: null,
  changePlan: null,
  validateResult: null,
  loading: false,
  erLoading: false,
  modelLoading: false,
  diffLoading: false,
  changePlanLoading: false,
  error: null,

  // 加载模块和模型列表
  loadModules: async () => {
    set({ loading: true, error: null })
    try {
      const res = await listModels()
      set({ modules: res.data, loading: false })
    } catch (err: any) {
      set({ error: err?.message || '加载模型列表失败', loading: false })
    }
  },

  // 加载 ER 图（可选 module 参数筛选指定模块）
  loadERDiagram: async (module?: string) => {
    set({ erLoading: true, error: null })
    try {
      const res = await getERDiagram(module)
      set({ erDsl: typeof res.data === 'string' ? res.data : (res.data as any)?.dsl || '', erLoading: false })
    } catch (err: any) {
      set({ error: err?.message || '加载 ER 图失败', erLoading: false })
    }
  },

  // 加载单个模型
  loadModel: async (module: string, name: string) => {
    set({ modelLoading: true, error: null })
    try {
      const res = await getModel(module, name)
      set({ currentModel: res.data, modelLoading: false })
    } catch (err: any) {
      set({ error: err?.message || '加载模型失败', modelLoading: false })
    }
  },

  // 保存模型（更新）
  saveModel: async (module: string, name: string, model: ModelIR) => {
    set({ error: null })
    try {
      const res = await updateModel(module, name, model)
      set({ currentModel: res.data })
      return res.data
    } catch (err: any) {
      set({ error: err?.message || '保存模型失败' })
      throw err
    }
  },

  // 创建新模型
  createNewModel: async (model: ModelIR) => {
    set({ error: null })
    try {
      const res = await createModel(model)
      set({ currentModel: res.data })
      return res.data
    } catch (err: any) {
      set({ error: err?.message || '创建模型失败' })
      throw err
    }
  },

  // 删除模型
  removeModel: async (module: string, name: string) => {
    set({ error: null })
    try {
      await deleteModel(module, name)
    } catch (err: any) {
      set({ error: err?.message || '删除模型失败' })
      throw err
    }
  },

  // 创建模块
  createNewModule: async (module: ModuleSummary) => {
    set({ error: null })
    try {
      const res = await createModule(module)
      return res.data
    } catch (err: any) {
      set({ error: err?.message || '创建模块失败' })
      throw err
    }
  },

  // 保存模块
  saveModule: async (name: string, module: ModuleSummary) => {
    set({ error: null })
    try {
      const res = await updateModule(name, module)
      return res.data
    } catch (err: any) {
      set({ error: err?.message || '保存模块失败' })
      throw err
    }
  },

  // 删除模块
  removeModule: async (name: string) => {
    set({ error: null })
    try {
      await deleteModule(name)
    } catch (err: any) {
      set({ error: err?.message || '删除模块失败' })
      throw err
    }
  },

  // 校验模型
  validateModelAction: async (module: string, name: string) => {
    set({ error: null })
    try {
      const res = await validateModel(module, name)
      set({ validateResult: res.data })
    } catch (err: any) {
      set({ error: err?.message || '校验模型失败' })
    }
  },

  // 加载差异
  loadDiff: async (module: string, name: string) => {
    set({ diffLoading: true, error: null, diffResult: null })
    try {
      const res = await getDiff(module, name)
      set({ diffResult: res.data, diffLoading: false })
    } catch (err: any) {
      set({ error: err?.message || '加载差异失败', diffLoading: false })
    }
  },

  // 加载 AI Coding 变更计划
  loadChangePlan: async (module: string, name: string) => {
    set({ changePlanLoading: true, error: null, changePlan: null })
    try {
      const res = await getChangePlan(module, name)
      set({ changePlan: res.data, changePlanLoading: false })
    } catch (err: any) {
      set({ error: err?.message || '加载 AI 变更计划失败', changePlanLoading: false })
    }
  },

  clearError: () => set({ error: null }),
  resetCurrentModel: () => set({ currentModel: null, validateResult: null, changePlan: null }),
}))
