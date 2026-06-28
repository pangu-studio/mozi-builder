import React, { useEffect, useState } from 'react'
import {
  Typography,
  Button,
  Space,
  Card,
  Tag,
  Spin,
  Breadcrumb,
  Empty,
  message,
  Descriptions,
  Alert,
} from 'antd'
import dayjs from 'dayjs'
import { ArrowLeftOutlined, CodeOutlined, ReloadOutlined, CheckCircleFilled } from '@ant-design/icons'
import { useNavigate, useParams } from 'react-router-dom'
import { useDevPlatformStore } from '../stores/dev-platform'
import { useMoziBuilder } from '../MoziBuilderProvider'
import { ChangeItem } from '../diffShared'

const { Text, Paragraph } = Typography

/** 将版本字符串（YYYYMMDDHHmmss）格式化为可读的本地时间 Tag */
const formatVersionTag = (v: string, color?: string) => {
  const t = dayjs(v, 'YYYYMMDDHHmmss')
  if (!t.isValid()) return <Tag color={color}>{v}</Tag>
  return <Tag color={color}>{t.format('YYYY-MM-DD HH:mm:ss')}</Tag>
}

const DiffViewer: React.FC = () => {
  const navigate = useNavigate()
  const { buildRoute } = useMoziBuilder()
  const { module, name } = useParams<{ module: string; name: string }>()
  const {
    diffResult,
    changePlan,
    diffLoading,
    changePlanLoading,
    error,
    loadDiff,
    loadChangePlan,
    clearError,
  } = useDevPlatformStore()

  useEffect(() => {
    if (module && name) {
      loadDiff(module, name)
      loadChangePlan(module, name)
    }
  }, [module, name])

  useEffect(() => {
    if (error) {
      message.error(error.split('\n')[0])
      clearError()
    }
  }, [error])

  const diff = diffResult
  const affectedFiles = changePlan?.affected_files || []
  const displayName = module && name ? `${module}/${name}` : ''

  // 统计
  const addedCount = diff?.changes.filter((c) => c.type === 'added').length || 0
  const removedCount = diff?.changes.filter((c) => c.type === 'removed').length || 0
  const modifiedCount = diff?.changes.filter((c) => c.type === 'modified').length || 0
  const hasPendingPlan = changePlan?.status === 'pending' && !!changePlan.prompt
  const iconDescriptionItems: NonNullable<React.ComponentProps<typeof Descriptions>['items']> = []
  if (changePlan?.module_icon) {
    iconDescriptionItems.push({ label: '模块图标', children: <Tag>{changePlan.module_icon}</Tag> })
  }
  if (changePlan?.model_icon) {
    iconDescriptionItems.push({ label: '模型图标', children: <Tag>{changePlan.model_icon}</Tag> })
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(buildRoute('/models'))}>
            返回
          </Button>
          <Breadcrumb
            items={[
              { title: '开发平台', onClick: () => navigate(buildRoute('/models')) },
              { title: `差异查看器：${displayName}` },
            ]}
          />
        </Space>
        <Space>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => {
              if (module && name) {
                loadDiff(module, name)
                loadChangePlan(module, name)
              }
            }}
            loading={diffLoading || changePlanLoading}
          >
            刷新
          </Button>
          <Button
            type="primary"
            icon={<CodeOutlined />}
            disabled={!hasPendingPlan}
            onClick={() => {
              if (!hasPendingPlan) {
                message.warning('当前模型没有待处理的 AI 变更计划')
                return
              }
              navigator.clipboard.writeText(changePlan.prompt)
              message.success('已复制 AI Coding Prompt')
            }}
          >
            复制 AI Prompt
          </Button>
        </Space>
      </div>

      {diffLoading ? (
        <div style={{ textAlign: 'center', padding: 80 }}>
          <Spin size="large" description="正在加载差异..." />
        </div>
      ) : !diff ? (
        <Empty description="暂无差异数据" />
      ) : changePlan?.status === 'applied' ? (
        <Card>
          <div style={{ textAlign: 'center', padding: '60px 20px' }}>
            <CheckCircleFilled style={{ fontSize: 48, color: '#52c41a', marginBottom: 16 }} />
            <div>
              <Text strong style={{ fontSize: 16 }}>
                模型 {displayName} 已同步
              </Text>
              <div style={{ marginTop: 8, color: '#52c41a' }}>
                当前版本 {diff.to_version} 的变更已应用到代码，无需重新生成
              </div>
              <div style={{ marginTop: 16 }}>
                <Button
                  type="primary"
                  onClick={() => {
                    navigator.clipboard.writeText(`mozi sync --model ${displayName}`)
                    message.success('已复制同步命令')
                  }}
                >
                  复制同步命令
                </Button>
              </div>
            </div>
          </div>
        </Card>
      ) : !diff.has_changes ? (
        <Card>
          <Empty description={`模型 ${displayName} 没有未生成的变更，当前版本 ${diff.to_version} 已是最新`} />
        </Card>
      ) : (
        <>
          {/* 变更概览 */}
          <Card size="small" style={{ marginBottom: 16 }}>
            <Descriptions
              title="变更概览"
              size="small"
              column={4}
              items={[
                { label: '模型', children: <Text code>{diff.model_ref}</Text> },
                { label: '从版本', children: formatVersionTag(diff.from_version) },
                { label: '到版本', children: formatVersionTag(diff.to_version, 'blue') },
              ]}
            />
            <Space size={16} style={{ marginTop: 12 }}>
              <Tag color="green">+ {addedCount} 新增</Tag>
              <Tag color="red">- {removedCount} 删除</Tag>
              <Tag color="orange">~ {modifiedCount} 修改</Tag>
            </Space>
            {changePlan?.requires_approval && <Alert type="warning" showIcon title="该计划包含破坏性或危险变更，应用前必须人工审批" style={{ marginTop: 12 }} />}
          </Card>

          <div style={{ display: 'flex', gap: 16 }}>
            {/* 左侧：变更列表 */}
            <div style={{ flex: 1 }}>
              <Card size="small" title={`变更详情（${diff.changes.length}）`} style={{ marginBottom: 16 }}>
                {diff.changes.map((change, i) => (
                  <ChangeItem
                    key={`${change.category}-${change.name}-${i}`}
                    change={change}
                    last={i === diff.changes.length - 1}
                  />
                ))}
              </Card>
            </div>

            {/* 右侧：受影响文件 */}
            <div style={{ width: 360, flexShrink: 0 }}>
              <Card
                size="small"
                title={`AI 变更计划${changePlanLoading ? '（加载中）' : ''}`}
                style={{ marginBottom: 16 }}
              >
                {changePlan ? (
                  <>
                    <Alert
                      type="info"
                      showIcon
                      title="模型驱动的增量修改"
                      description={changePlan.intent}
                      style={{ marginBottom: 12 }}
                    />
                    {iconDescriptionItems.length > 0 && (
                      <Descriptions
                        size="small"
                        column={1}
                        style={{ marginBottom: 12 }}
                        items={iconDescriptionItems}
                      />
                    )}
                    <Paragraph
                      copyable={{ text: changePlan.prompt }}
                      style={{
                        whiteSpace: 'pre-wrap',
                        background: '#fafafa',
                        border: '1px solid #f0f0f0',
                        borderRadius: 6,
                        padding: 10,
                        maxHeight: 220,
                        overflow: 'auto',
                        fontSize: 12,
                      }}
                    >
                      {changePlan.prompt}
                    </Paragraph>
                  </>
                ) : (
                  <Text type="secondary" style={{ fontSize: 13 }}>
                    暂无 AI 变更计划
                  </Text>
                )}
              </Card>

              <Card size="small" title={`受影响文件（${affectedFiles.length}）`} style={{ marginBottom: 16 }}>
                {affectedFiles.length > 0 ? (
                  affectedFiles.map((file, i) => (
                    <div
                      key={i}
                      style={{
                        padding: '8px 0',
                        borderBottom: i < affectedFiles.length - 1 ? '1px solid #f0f0f0' : 'none',
                      }}
                    >
                      <div style={{ fontFamily: 'monospace', fontSize: 12, marginBottom: 2, wordBreak: 'break-all' }}>
                        {file.path}
                      </div>
                      <Space size={4}>
                        <Text type="secondary" style={{ fontSize: 12 }}>
                          {file.description}
                        </Text>
                        <Tag style={{ fontSize: 10, lineHeight: '16px' }}>{file.change_count} 处变更</Tag>
                        <Tag color={file.evidence === 'certain' ? 'green' : file.evidence === 'suggested' ? 'purple' : 'orange'}>{file.evidence === 'certain' ? '确定' : file.evidence === 'suggested' ? 'AI 建议' : '规则推断'}</Tag>
                      </Space>
                    </div>
                  ))
                ) : (
                  <Text type="secondary" style={{ fontSize: 13 }}>
                    暂无受影响文件
                  </Text>
                )}
              </Card>

              {changePlan && (
                <>
                  {changePlan.migration?.steps?.length > 0 && (
                    <Card size="small" title="数据库迁移建议" style={{ marginBottom: 16 }}>
                      {changePlan.migration.has_dangerous && <Alert type="error" showIcon title="包含不可逆或数据丢失风险，禁止自动执行" style={{ marginBottom: 8 }} />}
                      {changePlan.migration.steps.map((step, i) => (
                        <div key={i} style={{ padding: '8px 0', borderBottom: i < changePlan.migration.steps.length - 1 ? '1px solid #f0f0f0' : 'none' }}>
                          <Tag color={step.risk === 'safe' ? 'green' : step.risk === 'dangerous' ? 'red' : 'orange'}>{step.risk}</Tag>
                          <Text>{step.description}</Text>
                          {step.sql && <Paragraph copyable={{ text: step.sql }} style={{ margin: '6px 0 0' }}><Text code>{step.sql}</Text></Paragraph>}
                        </div>
                      ))}
                    </Card>
                  )}
                  <Card size="small" title="执行任务" style={{ marginBottom: 16 }}>
                    {changePlan.tasks.map((task, i) => (
                      <div
                        key={i}
                        style={{
                          padding: '8px 0',
                          borderBottom: i < changePlan.tasks.length - 1 ? '1px solid #f0f0f0' : 'none',
                        }}
                      >
                        <Tag color="blue">{task.area}</Tag>
                        <Text style={{ fontSize: 13 }}>{task.description}</Text>
                      </div>
                    ))}
                  </Card>

                  <Card size="small" title="校验命令">
                    {changePlan.checks.map((check, i) => (
                      <Paragraph key={i} copyable={{ text: check }} style={{ marginBottom: 6 }}>
                        <Text code>{check}</Text>
                      </Paragraph>
                    ))}
                  </Card>
                </>
              )}
            </div>
          </div>
        </>
      )}
    </div>
  )
}

export default DiffViewer
