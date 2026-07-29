import React, { useEffect, useMemo, useState, useCallback } from 'react'
import { Button, Divider, Empty, Segmented, Select, Space, Spin, Tooltip, Typography } from 'antd'
import {
  ApartmentOutlined,
  FullscreenExitOutlined,
  FullscreenOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import ERDiagram from '../components/ERDiagram'
import type { ERTableDirection } from '../components/ERTableNode'
import { useDevPlatformStore } from '../stores/dev-platform'

const { Title, Text } = Typography

const ERDiagramPage: React.FC = () => {
  const { modules, erGraph, erGraphLoading, loadModules, loadERGraph } =
    useDevPlatformStore()
  const [selectedModule, setSelectedModule] = useState<string | undefined>(undefined)
  const [direction, setDirection] = useState<ERTableDirection>('LR')
  const [resetCount, setResetCount] = useState(0)
  const [fullscreen, setFullscreen] = useState(false)
  // 实体筛选：下拉值即实际显示的实体集合（所见即所得）。
  // null = 全部（默认）；[] = 一个都不显示
  const [selectedIds, setSelectedIds] = useState<string[] | null>(null)
  const [focusedId, setFocusedId] = useState<string | null>(null)

  // 首次加载模块列表
  useEffect(() => {
    if (modules.length === 0) {
      loadModules()
    }
  }, [])

  // 加载 ER 图（模块变化时重新加载）
  const fetchER = useCallback(() => {
    loadERGraph(selectedModule)
  }, [selectedModule, loadERGraph])

  useEffect(() => {
    fetchER()
  }, [fetchER])

  // 全屏时 Esc 退出
  useEffect(() => {
    if (!fullscreen) return
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setFullscreen(false)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [fullscreen])

  const allIds = useMemo(() => erGraph?.nodes.map((n) => n.id) ?? [], [erGraph])

  // 把一度关联实体并入给定集合（一跳扩展）
  const expandWithNeighbors = useCallback(
    (ids: Set<string>) => {
      const next = new Set(ids)
      if (!erGraph) return next
      for (const edge of erGraph.edges) {
        if (next.has(edge.source)) next.add(edge.target)
        if (next.has(edge.target)) next.add(edge.source)
      }
      return next
    },
    [erGraph],
  )

  const applySelection = useCallback(
    (ids: Set<string>) => {
      // 覆盖全部实体时归一为 null（全部）
      setSelectedIds(ids.size >= allIds.length ? null : [...ids])
    },
    [allIds],
  )

  const handleModuleChange = (value: string | undefined) => {
    setSelectedModule(value)
    // 切换模块后实体集合变化，重置筛选
    setSelectedIds(null)
    setFocusedId(null)
  }

  // 图上点选实体：聚焦该实体及其一度关联（结果可见于下拉，可继续修剪）；
  // 点空白处仅清除图内聚焦并恢复全图，不影响下拉中手动勾选的实体
  const handleEntityClick = useCallback(
    (id: string) => {
      applySelection(expandWithNeighbors(new Set([id])))
      setFocusedId(id)
    },
    [applySelection, expandWithNeighbors],
  )

  const handlePaneClick = useCallback(() => {
    // 仅当筛选来自图内点选聚焦时才恢复全图；手动勾选的筛选保持
    if (focusedId !== null) {
      setSelectedIds(null)
    }
    setFocusedId(null)
  }, [focusedId])

  const handleEntityFilterChange = (values: string[]) => {
    applySelection(new Set(values))
    setFocusedId(null)
  }

  const handleSelectAll = () => {
    setSelectedIds(null)
    setFocusedId(null)
  }

  const handleClearAll = () => {
    setSelectedIds([])
    setFocusedId(null)
  }

  const handleInvert = () => {
    const current = new Set(selectedIds ?? allIds)
    applySelection(new Set(allIds.filter((id) => !current.has(id))))
    setFocusedId(null)
  }

  const handleExpandNeighbors = () => {
    applySelection(expandWithNeighbors(new Set(selectedIds ?? allIds)))
  }

  // 过滤后的子图：仅保留选中实体与两端都可见的边。
  // ELK 会对子图重新布局，连线自动重新路由、变清爽
  const filteredGraph = useMemo(() => {
    if (!erGraph) return erGraph
    if (selectedIds === null) return erGraph
    const existing = new Set(allIds)
    const idSet = new Set(selectedIds.filter((id) => existing.has(id)))
    const nodes = erGraph.nodes.filter((n) => idSet.has(n.id))
    const nodeIds = new Set(nodes.map((n) => n.id))
    const edges = erGraph.edges.filter((e) => nodeIds.has(e.source) && nodeIds.has(e.target))
    return { nodes, edges }
  }, [erGraph, selectedIds, allIds])

  // 实体筛选下拉选项：按模块分组、可搜索
  const entityOptions = useMemo(() => {
    if (!erGraph) return []
    const groups = new Map<string, { label: string; options: { label: string; value: string }[] }>()
    for (const node of erGraph.nodes) {
      const mod = modules.find((m) => m.name === node.module)
      const groupLabel = mod ? `${mod.label} (${mod.name})` : node.module || '未分组'
      if (!groups.has(groupLabel)) groups.set(groupLabel, { label: groupLabel, options: [] })
      groups.get(groupLabel)!.options.push({
        label: node.label ? `${node.name} ${node.label}` : node.name,
        value: node.id,
      })
    }
    return [...groups.values()]
  }, [erGraph, modules])

  const filtering = selectedIds !== null

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: fullscreen ? '100vh' : 'calc(100vh - 160px)',
        // CSS 覆盖层全屏：避免原生 Fullscreen API 遮挡 antd 渲染到 body 的下拉/气泡
        ...(fullscreen
          ? {
              position: 'fixed' as const,
              inset: 0,
              zIndex: 1000,
              background: '#fff',
              padding: 16,
            }
          : {}),
      }}
    >
      {/* 头部 */}
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: 16,
          flexShrink: 0,
        }}
      >
        <Space align="center">
          <ApartmentOutlined style={{ fontSize: 24, color: '#667eea' }} />
          <Title level={4} style={{ margin: 0 }}>
            实体关系图
          </Title>
        </Space>
        <Space wrap>
          <Segmented
            value={direction}
            onChange={(value) => setDirection(value as ERTableDirection)}
            options={[
              { label: '横向', value: 'LR' },
              { label: '纵向', value: 'TB' },
            ]}
          />
          <Tooltip title="重置视图">
            <Button icon={<ReloadOutlined />} onClick={() => setResetCount((count) => count + 1)} />
          </Tooltip>
          <Tooltip title={fullscreen ? '退出全屏（Esc）' : '全屏显示'}>
            <Button
              icon={fullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />}
              onClick={() => setFullscreen((value) => !value)}
            />
          </Tooltip>
          <span style={{ color: '#666' }}>筛选模块：</span>
          <Select
            style={{ width: 200 }}
            placeholder="全部模块"
            allowClear
            value={selectedModule}
            onChange={handleModuleChange}
            options={[
              { label: '全部模块', value: undefined },
              ...modules.map((m) => ({
                label: `${m.label} (${m.name})`,
                value: m.name,
              })),
            ]}
          />
          <Tooltip title="勾选要显示的实体；也可直接在图上点击实体，聚焦它的一度关联">
            <span style={{ color: '#666' }}>筛选实体：</span>
          </Tooltip>
          <Select
            mode="multiple"
            style={{ minWidth: 200, maxWidth: 340 }}
            placeholder="选择实体"
            allowClear
            showSearch
            optionFilterProp="label"
            maxTagCount={2}
            maxTagPlaceholder={(omitted) => `+${omitted.length}`}
            value={selectedIds ?? allIds}
            onChange={handleEntityFilterChange}
            options={entityOptions}
            dropdownRender={(menu) => (
              <>
                {menu}
                <Divider style={{ margin: '4px 0' }} />
                <Space size={4} style={{ padding: '0 4px 4px' }}>
                  <Button type="link" size="small" onClick={handleSelectAll}>
                    全选
                  </Button>
                  <Button type="link" size="small" onClick={handleInvert}>
                    反选
                  </Button>
                  <Button type="link" size="small" onClick={handleClearAll}>
                    清空
                  </Button>
                  <Button type="link" size="small" onClick={handleExpandNeighbors}>
                    含一度关联
                  </Button>
                </Space>
              </>
            )}
          />
          {filtering && filteredGraph && erGraph && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              显示 {filteredGraph.nodes.length}/{erGraph.nodes.length} 个实体
              {focusedId ? '，点击图空白处恢复' : ''}
            </Text>
          )}
        </Space>
      </div>

      {/* 图表区域 — 占满剩余空间（React Flow 要求父容器有确定尺寸） */}
      <div
        style={{
          flex: 1,
          minHeight: 0,
          position: 'relative',
          overflow: 'hidden',
          border: '1px solid #f0f0f0',
          borderRadius: 8,
          background: '#fafafa',
        }}
      >
        {erGraphLoading ? (
          <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
            <Spin description="正在加载 ER 图..." />
          </div>
        ) : erGraph && erGraph.nodes.length > 0 ? (
          filteredGraph && filteredGraph.nodes.length > 0 ? (
            <ERDiagram
              key={resetCount}
              graph={filteredGraph}
              direction={direction}
              focusedId={focusedId}
              onEntityClick={handleEntityClick}
              onPaneClick={handlePaneClick}
            />
          ) : (
            <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
              <Empty description="未选择要显示的实体，请在右上角「筛选实体」中选择" />
            </div>
          )
        ) : (
          <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
            <Empty
              description={
                selectedModule
                  ? `模块 "${selectedModule}" 下暂无模型`
                  : '暂无 ER 图，请先导入或创建模型'
              }
            />
          </div>
        )}
      </div>
    </div>
  )
}

export default ERDiagramPage
