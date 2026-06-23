import React from 'react'
import { Select, Space, Typography } from 'antd'
import * as AntIcons from '@ant-design/icons'

const { Text } = Typography

const COMMON_ICON_NAMES = [
  'AppstoreOutlined',
  'DashboardOutlined',
  'UserOutlined',
  'TeamOutlined',
  'FileTextOutlined',
  'BookOutlined',
  'DatabaseOutlined',
  'FolderOutlined',
  'TagsOutlined',
  'SettingOutlined',
  'ProfileOutlined',
  'FormOutlined',
  'TableOutlined',
  'CalendarOutlined',
  'ClockCircleOutlined',
  'NotificationOutlined',
  'MessageOutlined',
  'BellOutlined',
  'SearchOutlined',
  'BarChartOutlined',
  'LineChartOutlined',
  'PieChartOutlined',
  'ExperimentOutlined',
  'DeploymentUnitOutlined',
  'ApiOutlined',
  'CloudOutlined',
  'SafetyCertificateOutlined',
  'LockOutlined',
  'KeyOutlined',
  'AuditOutlined',
]

const iconComponentMap = AntIcons as unknown as Record<string, React.ComponentType>

const ALL_ICON_NAMES = Object.keys(iconComponentMap)
  .filter((name) => name.endsWith('Outlined'))
  .sort()

export const ICON_OPTIONS = [
  ...COMMON_ICON_NAMES,
  ...ALL_ICON_NAMES.filter((name) => !COMMON_ICON_NAMES.includes(name)),
].map((name) => ({ value: name, label: name }))

function renderIcon(name?: string) {
  if (!name) return null
  const Icon = iconComponentMap[name]
  return Icon ? <Icon /> : <AppstoreFallback />
}

const AppstoreFallback = iconComponentMap.AppstoreOutlined || (() => null)

interface IconSelectProps {
  value?: string
  onChange?: (value?: string) => void
  placeholder?: string
  disabled?: boolean
}

const IconSelect: React.FC<IconSelectProps> = ({ value, onChange, placeholder = '选择图标', disabled }) => {
  const options = ICON_OPTIONS.some((item) => item.value === value) || !value
    ? ICON_OPTIONS
    : [{ value, label: value }, ...ICON_OPTIONS]

  return (
    <Select
      showSearch
      allowClear
      disabled={disabled}
      style={{ width: '100%' }}
      placeholder={placeholder}
      value={value || undefined}
      options={options}
      optionFilterProp="label"
      onChange={(next) => onChange?.(next)}
      optionRender={(option) => (
        <Space>
          {renderIcon(String(option.value))}
          <span>{option.label}</span>
        </Space>
      )}
      labelRender={(option) => (
        <Space size={6}>
          {renderIcon(String(option.value))}
          <Text>{option.label}</Text>
        </Space>
      )}
    />
  )
}

export default IconSelect
