import React, { useEffect, useState, useCallback } from 'react'
import { Button, Empty, Segmented, Select, Space, Spin, Tooltip, Typography } from 'antd'
import {
  ApartmentOutlined,
  ReloadOutlined,
  ZoomInOutlined,
  ZoomOutOutlined,
} from '@ant-design/icons'
import ERDiagram, { type ERDiagramDirection } from '../components/ERDiagram'
import { useDevPlatformStore } from '../stores/dev-platform'

const { Title } = Typography

const ERDiagramPage: React.FC = () => {
  const { modules, erDsl, erLoading, loadModules, loadERDiagram } =
    useDevPlatformStore()
  const [selectedModule, setSelectedModule] = useState<string | undefined>(undefined)
  const [direction, setDirection] = useState<ERDiagramDirection>('LR')
  const [zoom, setZoom] = useState(100)

  // 首次加载模块列表
  useEffect(() => {
    if (modules.length === 0) {
      loadModules()
    }
  }, [])

  // 加载 ER 图（模块变化时重新加载）
  const fetchER = useCallback(() => {
    loadERDiagram(selectedModule)
  }, [selectedModule, loadERDiagram])

  useEffect(() => {
    fetchER()
  }, [fetchER])

  const handleModuleChange = (value: string | undefined) => {
    setSelectedModule(value)
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100vh - 160px)' }}>
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
            onChange={(value) => setDirection(value as ERDiagramDirection)}
            options={[
              { label: '横向', value: 'LR' },
              { label: '纵向', value: 'TB' },
            ]}
          />
          <Space.Compact>
            <Tooltip title="缩小">
              <Button
                icon={<ZoomOutOutlined />}
                disabled={zoom <= 60}
                onClick={() => setZoom((value) => Math.max(60, value - 10))}
              />
            </Tooltip>
            <Button style={{ width: 64 }} onClick={() => setZoom(100)}>
              {zoom}%
            </Button>
            <Tooltip title="放大">
              <Button
                icon={<ZoomInOutlined />}
                disabled={zoom >= 180}
                onClick={() => setZoom((value) => Math.min(180, value + 10))}
              />
            </Tooltip>
          </Space.Compact>
          <Tooltip title="重置视图">
            <Button icon={<ReloadOutlined />} onClick={() => { setDirection('LR'); setZoom(100) }} />
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
        </Space>
      </div>

      {/* 图表区域 — 占满剩余空间 */}
      <div
        style={{
          flex: 1,
          minHeight: 0,
          border: '1px solid #f0f0f0',
          borderRadius: 8,
          background: '#fafafa',
          overflow: 'auto',
        }}
      >
        {erLoading ? (
          <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
            <Spin tip="正在加载 ER 图..." />
          </div>
        ) : erDsl ? (
          <ERDiagram dsl={erDsl} loading={false} direction={direction} zoom={zoom} />
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
