import React, { useEffect, useRef } from 'react'
import { Spin, Typography } from 'antd'
import mermaid from 'mermaid'

const { Text } = Typography

export type ERDiagramDirection = 'LR' | 'TB'

let mermaidInitialized = false
function initMermaid() {
  if (mermaidInitialized) return
  mermaid.initialize({
    startOnLoad: false,
    theme: 'base',
    securityLevel: 'loose',
    er: {
      useMaxWidth: false,
      layoutDirection: 'LR',
    },
    themeVariables: {
      primaryColor: '#667eea',
      primaryTextColor: '#333',
      lineColor: '#999',
    },
  })
  mermaidInitialized = true
}

interface ERDiagramProps {
  dsl: string
  loading?: boolean
  direction?: ERDiagramDirection
  zoom?: number
}

function applyLayoutDirection(dsl: string, direction: ERDiagramDirection) {
  const lines = dsl
    .replace(/(layoutDirection['"]?\s*:\s*['"])(TB|BT|LR|RL)(['"])/i, `$1${direction}$3`)
    .split('\n')
    .filter((line) => !/^\s*direction\s+(TB|BT|LR|RL)\s*$/i.test(line))

  const diagramLineIndex = lines.findIndex((line) => /^\s*erDiagram\b/i.test(line))
  if (diagramLineIndex >= 0) {
    lines.splice(diagramLineIndex + 1, 0, `    direction ${direction}`)
  }

  return lines.join('\n')
}

function resizeSvg(container: HTMLDivElement, zoom: number) {
  const svg = container.querySelector('svg')
  if (!svg) return

  svg.style.maxWidth = 'none'
  svg.style.display = 'block'
  svg.style.margin = '0 auto'

  const viewBox = svg.getAttribute('viewBox')?.trim().split(/\s+/).map(Number)
  if (!viewBox || viewBox.length !== 4 || viewBox.some(Number.isNaN)) return

  const scale = zoom / 100
  svg.setAttribute('width', String(Math.ceil(viewBox[2] * scale)))
  svg.setAttribute('height', String(Math.ceil(viewBox[3] * scale)))
}

const ERDiagram: React.FC<ERDiagramProps> = ({ dsl, loading, direction = 'LR', zoom = 100 }) => {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!dsl) return
    initMermaid()

    const renderDiagram = async () => {
      try {
        const id = `er-diagram-${Date.now()}`
        const { svg } = await mermaid.render(id, applyLayoutDirection(dsl, direction))
        if (containerRef.current) {
          containerRef.current.innerHTML = svg
          resizeSvg(containerRef.current, zoom)
        }
      } catch (err) {
        console.error('Mermaid render error:', err)
        if (containerRef.current) {
          containerRef.current.innerHTML = ''
        }
      }
    }

    renderDiagram()
  }, [dsl, direction, zoom])

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

  if (!dsl) {
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
    <div
      ref={containerRef}
      style={{
        padding: 24,
        display: 'inline-block',
        minWidth: '100%',
        minHeight: '100%',
      }}
    />
  )
}

export default ERDiagram
