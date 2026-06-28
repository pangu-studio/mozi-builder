import React from 'react'
import { Route } from 'react-router-dom'
import APIWorkbench from './pages/APIWorkbench'
import DiffViewer from './pages/DiffViewer'
import ERDiagramPage from './pages/ERDiagramPage'
import ErrorCodeManager from './pages/ErrorCodeManager'
import Guide from './pages/Guide'
import ModelDesigner from './pages/ModelDesigner'
import ModelOverview from './pages/ModelOverview'

// Child routes for a host <Route path="...">. Keeping this as route elements
// lets host apps choose their own parent path and layout.
export function BuilderRoutes() {
  return (
    <>
      <Route path="models" element={<ModelOverview />} />
      <Route path="apis" element={<APIWorkbench />} />
      <Route path="error-codes" element={<ErrorCodeManager />} />
      <Route path="guide" element={<Guide />} />
      <Route path="models/new" element={<ModelDesigner />} />
      <Route path="er" element={<ERDiagramPage />} />
      <Route path="modules/:module/models/:name" element={<ModelDesigner />} />
      <Route path="modules/:module/models/:name/diff" element={<DiffViewer />} />
    </>
  )
}
