import React, { useEffect, useMemo, useState } from 'react'
import { Spin, Typography } from 'antd'
import {
  Background,
  Controls,
  MarkerType,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  type NodeTypes,
  type EdgeTypes,
} from '@xyflow/react'
import ELK from 'elkjs/lib/elk.bundled.js'
import type { ElkExtendedEdge } from 'elkjs'
import '@xyflow/react/dist/style.css'
import type { ERGraph } from '../api/dev-platform'
import ERTableNode, {
  HEADER_HEIGHT,
  NODE_WIDTH,
  ROW_HEIGHT,
  type ERTableDirection,
  type ERTableNodeType,
} from './ERTableNode'
import ERRelationEdge, { type ERRelationEdgeType } from './ERRelationEdge'

const { Text } = Typography

const nodeTypes: NodeTypes = { erTable: ERTableNode }
const edgeTypes: EdgeTypes = { erRelation: ERRelationEdge }

interface ERDiagramProps {
  graph: ERGraph | null
  loading?: boolean
  direction?: ERTableDirection
  focusedId?: string | null
  onEntityClick?: (id: string) => void
  onPaneClick?: () => void
}

type XY = { x: number; y: number }

// 计算与前端 Handle 一一对应的 elk 端口（id 形如 "Node/f:deck_id:s"），
// 坐标用固定尺寸常量精确推导，与 ERTableNode 的实际渲染保持一致
function nodePorts(node: ERGraph['nodes'][number], direction: ERTableDirection) {
  const horizontal = direction !== 'TB'
  const ports: { id: string; x: number; y: number; width: number; height: number }[] = []
  const addPort = (handleId: string, x: number, y: number) =>
    ports.push({ id: `${node.id}/${handleId}`, x, y, width: 1, height: 1 })

  if (horizontal) {
    addPort('t:t', 0, HEADER_HEIGHT / 2)
    addPort('t:s', NODE_WIDTH, HEADER_HEIGHT / 2)
    node.fields.forEach((field, index) => {
      const y = HEADER_HEIGHT + index * ROW_HEIGHT + ROW_HEIGHT / 2
      addPort(`f:${field.name}:t`, 0, y)
      addPort(`f:${field.name}:s`, NODE_WIDTH, y)
    })
  } else {
    addPort('t:t', NODE_WIDTH / 2, 0)
    addPort('t:s', NODE_WIDTH / 2, HEADER_HEIGHT)
    node.fields.forEach((field, index) => {
      const rowTop = HEADER_HEIGHT + index * ROW_HEIGHT
      addPort(`f:${field.name}:t`, NODE_WIDTH / 2, rowTop)
      addPort(`f:${field.name}:s`, NODE_WIDTH / 2, rowTop + ROW_HEIGHT)
    })
  }
  return ports
}

// 用 elk 计算节点坐标与正交边路由（绕开节点），边折线存进 edge.data.points
async function layoutGraph(graph: ERGraph, direction: ERTableDirection) {
  // 节点实际声明的字段集合（含后端注入的 FK）：端点若指向未渲染的字段，
  // 降级为表级锚点，避免 React Flow 因找不到 handle 而丢弃整条边
  const declaredFields = new Map(
    graph.nodes.map((node) => [node.id, new Set(node.fields.map((field) => field.name))]),
  )
  const fieldHandle = (nodeId: string, field: string, suffix: 's' | 't', fallback: string) =>
    field && declaredFields.get(nodeId)?.has(field) ? `f:${field}:${suffix}` : fallback

  const handlePairs = graph.edges.map((edge) => ({
    edge,
    sourceHandle: fieldHandle(edge.source, edge.source_field, 's', 't:s'),
    targetHandle: fieldHandle(edge.target, edge.target_field, 't', 't:t'),
  }))

  const elk = new ELK()
  const result = await elk.layout({
    id: 'root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': direction === 'LR' ? 'RIGHT' : 'DOWN',
      'elk.edgeRouting': 'ORTHOGONAL',
      'elk.spacing.nodeNodeBetweenLayers': '140',
      'elk.spacing.nodeNode': '80',
      // 拉开边与边、边与节点的间距，给关系标签留出走廊，避免平行边标签重叠
      'elk.spacing.edgeEdge': '20',
      'elk.spacing.edgeNode': '24',
      'elk.layered.spacing.edgeEdgeBetweenLayers': '16',
      'elk.layered.spacing.edgeNodeBetweenLayers': '24',
    },
    children: graph.nodes.map((node) => ({
      id: node.id,
      width: NODE_WIDTH,
      height: HEADER_HEIGHT + node.fields.length * ROW_HEIGHT,
      layoutOptions: {
        'elk.portConstraints': 'FIXED_POS',
      },
      ports: nodePorts(node, direction),
    })),
    edges: handlePairs.map(({ edge, sourceHandle, targetHandle }) => ({
      id: edge.id,
      sources: [`${edge.source}/${sourceHandle}`],
      targets: [`${edge.target}/${targetHandle}`],
    })),
  })

  const positions = new Map(
    (result.children ?? []).map((child) => [child.id, { x: child.x ?? 0, y: child.y ?? 0 }]),
  )

  // elk 边路由结果：sections[0] 与节点坐标同一坐标系
  const routedPoints = new Map<string, XY[]>()
  for (const elkEdge of (result.edges ?? []) as ElkExtendedEdge[]) {
    const section = elkEdge.sections?.[0]
    if (!section) continue
    routedPoints.set(elkEdge.id, [
      section.startPoint,
      ...(section.bendPoints ?? []),
      section.endPoint,
    ])
  }

  const nodes: ERTableNodeType[] = graph.nodes.map((node) => ({
    id: node.id,
    type: 'erTable' as const,
    position: positions.get(node.id) ?? { x: 0, y: 0 },
    // 预设宽高使 fitView 无需等待 ResizeObserver 测量
    width: NODE_WIDTH,
    height: HEADER_HEIGHT + node.fields.length * ROW_HEIGHT,
    data: {
      name: node.name,
      label: node.label,
      table: node.table,
      fields: node.fields,
      direction,
    },
    draggable: false,
    connectable: false,
  }))

  const edges: ERRelationEdgeType[] = handlePairs.map(({ edge, sourceHandle, targetHandle }) => ({
    id: edge.id,
    source: edge.source,
    target: edge.target,
    type: 'erRelation' as const,
    sourceHandle,
    targetHandle,
    data: {
      label: edge.label,
      sourceCard: edge.source_card,
      targetCard: edge.target_card,
      points: routedPoints.get(edge.id),
    },
    markerEnd: { type: MarkerType.ArrowClosed, color: '#a3a3c2', width: 16, height: 16 },
  }))

  return { nodes, edges }
}

const ERDiagramFlow: React.FC<{
  graph: ERGraph
  direction: ERTableDirection
  focusedId?: string | null
  onEntityClick?: (id: string) => void
  onPaneClick?: () => void
}> = ({ graph, direction, focusedId, onEntityClick, onPaneClick }) => {
  const [nodes, setNodes] = useState<ERTableNodeType[]>([])
  const [edges, setEdges] = useState<ERRelationEdgeType[]>([])
  const [layouting, setLayouting] = useState(true)
  const { fitView } = useReactFlow()

  // 聚焦高亮只更新节点 data，不触发 elk 重新布局
  const displayNodes = useMemo(
    () =>
      nodes.map((node) => ({
        ...node,
        data: { ...node.data, focused: node.id === focusedId },
      })),
    [nodes, focusedId],
  )

  // 数据或方向变化时重新跑 elk 布局
  useEffect(() => {
    let cancelled = false
    setLayouting(true)

    layoutGraph(graph, direction)
      .then(({ nodes: nextNodes, edges: nextEdges }) => {
        if (cancelled) return
        setNodes(nextNodes)
        setEdges(nextEdges)
        setLayouting(false)
      })
      .catch((err) => {
        if (cancelled) return
        console.error('ELK layout error:', err)
        setNodes([])
        setEdges([])
        setLayouting(false)
      })

    return () => {
      cancelled = true
    }
  }, [graph, direction])

  // 布局完成后适配视图（节点已带固定宽高，可直接计算）
  useEffect(() => {
    if (layouting || nodes.length === 0) return
    const timer = setTimeout(() => {
      fitView({ padding: 0.15, maxZoom: 1.2 })
    }, 0)
    return () => clearTimeout(timer)
  }, [layouting, nodes, fitView])

  if (layouting) {
    return (
      <div
        style={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          height: '100%',
        }}
      >
        <Spin description="正在布局 ER 图..." />
      </div>
    )
  }

  return (
    <ReactFlow
      nodes={displayNodes}
      edges={edges}
      nodeTypes={nodeTypes}
      edgeTypes={edgeTypes}
      fitView
      fitViewOptions={{ padding: 0.15, maxZoom: 1.2 }}
      nodesDraggable={false}
      nodesConnectable={false}
      onNodeClick={(_, node) => onEntityClick?.(node.id)}
      onPaneClick={() => onPaneClick?.()}
    >
      <Background />
      <Controls />
    </ReactFlow>
  )
}

const ERDiagram: React.FC<ERDiagramProps> = ({
  graph,
  loading,
  direction = 'LR',
  focusedId,
  onEntityClick,
  onPaneClick,
}) => {
  if (loading) {
    return (
      <div
        style={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          height: '100%',
        }}
      >
        <Spin description="正在加载 ER 图..." />
      </div>
    )
  }

  if (!graph || graph.nodes.length === 0) {
    return (
      <div
        style={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          height: '100%',
          color: '#999',
        }}
      >
        <Text type="secondary">暂无 ER 图，请先导入或创建模型</Text>
      </div>
    )
  }

  return (
    <div style={{ width: '100%', height: '100%' }}>
      <ReactFlowProvider>
        <ERDiagramFlow
          graph={graph}
          direction={direction}
          focusedId={focusedId}
          onEntityClick={onEntityClick}
          onPaneClick={onPaneClick}
        />
      </ReactFlowProvider>
    </div>
  )
}

export default ERDiagram
