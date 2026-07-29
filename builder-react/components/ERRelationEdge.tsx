import React from 'react'
import {
  BaseEdge,
  EdgeLabelRenderer,
  getSmoothStepPath,
  type Edge,
  type EdgeProps,
} from '@xyflow/react'

export type ERPoint = { x: number; y: number }

export type ERRelationEdgeData = {
  label?: string
  sourceCard?: string
  targetCard?: string
  // elk 正交路由的折线顶点（含首尾端点），与节点坐标同一坐标系
  points?: ERPoint[]
}

export type ERRelationEdgeType = Edge<ERRelationEdgeData, 'erRelation'>

// 中心标签离线的垂直距离（px）：放到线的侧面，避免压线与端点拥挤
const CENTER_LABEL_OFFSET = 12

const endpointLabelStyle: React.CSSProperties = {
  position: 'absolute',
  transform: 'translate(-50%, -50%)',
  fontSize: 11,
  fontWeight: 600,
  color: '#667eea',
  background: '#fff',
  padding: '0 3px',
  borderRadius: 3,
  pointerEvents: 'none',
}

const centerLabelStyle: React.CSSProperties = {
  position: 'absolute',
  transform: 'translate(-50%, -50%)',
  fontSize: 11,
  color: '#666',
  background: '#fff',
  padding: '1px 6px',
  borderRadius: 4,
  border: '1px solid #eee',
  whiteSpace: 'nowrap',
  pointerEvents: 'none',
}

// 折线路径：M 起点 + L 各顶点
function polylinePath(points: ERPoint[]) {
  return points
    .map((point, index) => `${index === 0 ? 'M' : 'L'}${point.x},${point.y}`)
    .join(' ')
}

// 取折线最长段的中点与该段方向：最长段所在的走廊最宽敞，
// 标签放这里比放整条路径中点更不容易挤在节点边界或拐角处
function longestSegmentMidpoint(points: ERPoint[]): { mid: ERPoint; dir: ERPoint } {
  let best = { mid: points[0], dir: { x: 1, y: 0 }, len: -1 }
  for (let i = 1; i < points.length; i++) {
    const a = points[i - 1]
    const b = points[i]
    const len = Math.hypot(b.x - a.x, b.y - a.y)
    if (len > best.len) {
      best = {
        mid: { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 },
        dir: segmentDirection(a, b),
        len,
      }
    }
  }
  return best
}

// 沿首段/末段方向的单位向量
function segmentDirection(from: ERPoint, to: ERPoint): ERPoint {
  const len = Math.hypot(to.x - from.x, to.y - from.y) || 1
  return { x: (to.x - from.x) / len, y: (to.y - from.y) / len }
}

const ERRelationEdge: React.FC<EdgeProps<ERRelationEdgeType>> = ({
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
  markerEnd,
}) => {
  const points = data?.points && data.points.length >= 2 ? data.points : undefined

  let edgePath: string
  let labelX: number
  let labelY: number
  let sourcePoint: ERPoint
  let sourceDir: ERPoint
  let targetPoint: ERPoint
  let targetDir: ERPoint
  // 端点基数标签沿首/末段的偏移量：钳制在段长一半以内，短边上不冲出拐角
  let sourceLabelOffset = 18
  let targetLabelOffset = 18

  if (points) {
    // elk 折线路由
    edgePath = polylinePath(points)
    const { mid, dir } = longestSegmentMidpoint(points)
    // 标签沿最长段的垂直方向偏移，放到线的侧面而不是压在线上，
    // 同时避开短边端点处与基数标签的拥挤区
    labelX = mid.x + dir.y * CENTER_LABEL_OFFSET
    labelY = mid.y - dir.x * CENTER_LABEL_OFFSET
    sourcePoint = points[0]
    sourceDir = segmentDirection(points[0], points[1])
    targetPoint = points[points.length - 1]
    targetDir = segmentDirection(points[points.length - 2], points[points.length - 1])
    const sourceSegLen = Math.hypot(points[1].x - points[0].x, points[1].y - points[0].y)
    const targetSegLen = Math.hypot(
      targetPoint.x - points[points.length - 2].x,
      targetPoint.y - points[points.length - 2].y,
    )
    sourceLabelOffset = Math.min(18, sourceSegLen / 2)
    targetLabelOffset = Math.min(18, targetSegLen / 2)
  } else {
    // 无路由数据时回退 smoothstep（防御）
    ;[edgePath, labelX, labelY] = getSmoothStepPath({
      sourceX,
      sourceY,
      targetX,
      targetY,
      sourcePosition,
      targetPosition,
      borderRadius: 8,
    })
    sourcePoint = { x: sourceX, y: sourceY }
    targetPoint = { x: targetX, y: targetY }
    const dir = segmentDirection(sourcePoint, targetPoint)
    sourceDir = dir
    targetDir = dir
  }

  return (
    <>
      <BaseEdge
        path={edgePath}
        markerEnd={markerEnd}
        style={{ stroke: '#a3a3c2', strokeWidth: 1.5 }}
      />
      <EdgeLabelRenderer>
        {data?.sourceCard && (
          <div
            style={{
              ...endpointLabelStyle,
              left: sourcePoint.x + sourceDir.x * sourceLabelOffset,
              top: sourcePoint.y + sourceDir.y * sourceLabelOffset,
            }}
          >
            {data.sourceCard}
          </div>
        )}
        {data?.targetCard && (
          <div
            style={{
              ...endpointLabelStyle,
              left: targetPoint.x - targetDir.x * targetLabelOffset,
              top: targetPoint.y - targetDir.y * targetLabelOffset,
            }}
          >
            {data.targetCard}
          </div>
        )}
        {data?.label && (
          <div style={{ ...centerLabelStyle, left: labelX, top: labelY }}>{data.label}</div>
        )}
      </EdgeLabelRenderer>
    </>
  )
}

export default ERRelationEdge
