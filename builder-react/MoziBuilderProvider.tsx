import React, { createContext, useContext } from 'react'
import type { AxiosInstance } from 'axios'
import {
  configureMoziBuilderApi,
  getMoziBuilderApiBasePath,
} from './api/dev-platform'

export interface MoziBuilderContextValue {
  apiClient: AxiosInstance | null
  apiBasePath: string
  routeBasePath: string
  guideMarkdown: string
  buildRoute: (path?: string) => string
}

export interface MoziBuilderProviderProps {
  apiClient: AxiosInstance
  apiBasePath?: string
  routeBasePath?: string
  guideMarkdown?: string
  children: React.ReactNode
}

const defaultGuideMarkdown = `# Mozi Builder

Mozi Builder is embedded by the host application.

- AI agents and skills should use the mozi CLI.
- The Builder UI is for human review, visual editing, API Workbench, and lightweight curation.
- The host application provides API client, routing, authentication, and deployment.
`

const MoziBuilderContext = createContext<MoziBuilderContextValue>({
  apiClient: null,
  apiBasePath: getMoziBuilderApiBasePath(),
  routeBasePath: '/dev-platform',
  guideMarkdown: defaultGuideMarkdown,
  buildRoute: (path = '') => buildRoute('/dev-platform', path),
})

export function MoziBuilderProvider({
  apiClient,
  apiBasePath = '/dev-platform',
  routeBasePath = '/dev-platform',
  guideMarkdown = defaultGuideMarkdown,
  children,
}: MoziBuilderProviderProps) {
  configureMoziBuilderApi({ client: apiClient, basePath: apiBasePath })
  const normalizedRouteBasePath = normalizeRouteBasePath(routeBasePath)

  return (
    <MoziBuilderContext.Provider
      value={{
        apiClient,
        apiBasePath,
        routeBasePath: normalizedRouteBasePath,
        guideMarkdown,
        buildRoute: (path = '') => buildRoute(normalizedRouteBasePath, path),
      }}
    >
      {children}
    </MoziBuilderContext.Provider>
  )
}

export function useMoziBuilder() {
  return useContext(MoziBuilderContext)
}

function normalizeRouteBasePath(path: string) {
  const trimmed = path.trim()
  if (!trimmed || trimmed === '/') return ''
  const withLeadingSlash = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  return withLeadingSlash.endsWith('/') ? withLeadingSlash.slice(0, -1) : withLeadingSlash
}

function buildRoute(basePath: string, path = '') {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  if (!basePath) return normalizedPath
  if (normalizedPath === '/') return basePath
  return `${basePath}${normalizedPath}`
}
