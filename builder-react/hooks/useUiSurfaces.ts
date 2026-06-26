import { useState, useEffect } from 'react'
import { listDesignDictionaryItems, type DesignDictionaryItem } from '../api/dev-platform'

const UI_SURFACES_DICTIONARY = 'ui_surfaces'

/**
 * Hook that fetches the ui_surfaces design dictionary and returns
 * sorted, enabled surface items plus a label-lookup helper.
 *
 * Usage:
 *   const { surfaces, labelOf } = useUiSurfaces()
 *   // surfaces = [{ value: 'admin', label: '管理后台', ... }, ...]
 *   // labelOf('cli') → '命令行 / CLI（AI Agent 调用）'
 */
export function useUiSurfaces() {
  const [items, setItems] = useState<DesignDictionaryItem[]>([])

  useEffect(() => {
    let cancelled = false
    listDesignDictionaryItems(UI_SURFACES_DICTIONARY, false)
      .then((res) => {
        if (!cancelled) setItems(res.data || [])
      })
      .catch(() => {
        if (!cancelled) setItems([])
      })
    return () => { cancelled = true }
  }, [])

  const surfaces = items
    .filter((item) => item.enabled !== false)
    .sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))

  const labelOf = (value: string): string => {
    const item = items.find((i) => i.value === value)
    return item?.label || value
  }

  return { surfaces, labelOf }
}
