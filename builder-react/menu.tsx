import React from 'react'
import {
  ApartmentOutlined,
  ApiOutlined,
  WarningOutlined,
  NodeIndexOutlined,
  ReadOutlined,
  ToolOutlined,
} from '@ant-design/icons'

export function createMoziBuilderMenuItem(routeBasePath = '/dev-platform') {
  const basePath = normalizeRouteBasePath(routeBasePath)
  return {
    key: 'dev-platform',
    icon: <ToolOutlined />,
    label: '开发平台',
    children: [
      { key: `${basePath}/models`, icon: <ApartmentOutlined />, label: '模型管理' },
      { key: `${basePath}/apis`, icon: <ApiOutlined />, label: 'API Workbench' },
      { key: `${basePath}/error-codes`, icon: <WarningOutlined />, label: '错误码管理' },
      { key: `${basePath}/er`, icon: <NodeIndexOutlined />, label: '实体关系图' },
      { key: `${basePath}/guide`, icon: <ReadOutlined />, label: '操作手册' },
    ],
  }
}

export const moziBuilderMenuItem = createMoziBuilderMenuItem()

function normalizeRouteBasePath(path: string) {
  const trimmed = path.trim()
  if (!trimmed || trimmed === '/') return ''
  const withLeadingSlash = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  return withLeadingSlash.endsWith('/') ? withLeadingSlash.slice(0, -1) : withLeadingSlash
}
