import React from 'react'
import { Typography, theme } from 'antd'
import { useMoziBuilder } from '../MoziBuilderProvider'

const { Title, Text } = Typography

type MarkdownBlock =
  | { type: 'heading'; level: number; text: string }
  | { type: 'paragraph'; text: string }
  | { type: 'unorderedList'; items: string[] }
  | { type: 'orderedList'; items: string[] }
  | { type: 'code'; lang: string; code: string }
  | { type: 'table'; headers: string[]; rows: string[][] }

const isTableSeparator = (line: string) =>
  /^\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?$/.test(line)

const splitTableRow = (line: string) =>
  line
    .trim()
    .replace(/^\|/, '')
    .replace(/\|$/, '')
    .split('|')
    .map((cell) => cell.trim())

const parseMarkdown = (markdown: string): MarkdownBlock[] => {
  const blocks: MarkdownBlock[] = []
  const lines = markdown.replace(/\r\n/g, '\n').split('\n')
  let i = 0

  while (i < lines.length) {
    const line = lines[i]

    if (!line.trim()) {
      i += 1
      continue
    }

    const codeMatch = line.match(/^```(\w+)?\s*$/)
    if (codeMatch) {
      const codeLines: string[] = []
      i += 1
      while (i < lines.length && !lines[i].startsWith('```')) {
        codeLines.push(lines[i])
        i += 1
      }
      blocks.push({ type: 'code', lang: codeMatch[1] || '', code: codeLines.join('\n') })
      i += 1
      continue
    }

    const headingMatch = line.match(/^(#{1,4})\s+(.+)$/)
    if (headingMatch) {
      blocks.push({
        type: 'heading',
        level: headingMatch[1].length,
        text: headingMatch[2].trim(),
      })
      i += 1
      continue
    }

    if (line.includes('|') && i + 1 < lines.length && isTableSeparator(lines[i + 1])) {
      const headers = splitTableRow(line)
      const rows: string[][] = []
      i += 2
      while (i < lines.length && lines[i].includes('|') && lines[i].trim()) {
        rows.push(splitTableRow(lines[i]))
        i += 1
      }
      blocks.push({ type: 'table', headers, rows })
      continue
    }

    if (/^\s*-\s+/.test(line)) {
      const items: string[] = []
      while (i < lines.length && /^\s*-\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*-\s+/, '').trim())
        i += 1
      }
      blocks.push({ type: 'unorderedList', items })
      continue
    }

    if (/^\s*\d+\.\s+/.test(line)) {
      const items: string[] = []
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*\d+\.\s+/, '').trim())
        i += 1
      }
      blocks.push({ type: 'orderedList', items })
      continue
    }

    const paragraphLines = [line.trim()]
    i += 1
    while (
      i < lines.length &&
      lines[i].trim() &&
      !/^```/.test(lines[i]) &&
      !/^(#{1,4})\s+/.test(lines[i]) &&
      !/^\s*[-\d]/.test(lines[i]) &&
      !(lines[i].includes('|') && i + 1 < lines.length && isTableSeparator(lines[i + 1]))
    ) {
      paragraphLines.push(lines[i].trim())
      i += 1
    }
    blocks.push({ type: 'paragraph', text: paragraphLines.join(' ') })
  }

  return blocks
}

const renderInline = (text: string) => {
  const parts = text.split(/(`[^`]+`|\*\*[^*]+\*\*)/g)

  return parts.map((part, index) => {
    if (part.startsWith('`') && part.endsWith('`')) {
      return <Text code key={index}>{part.slice(1, -1)}</Text>
    }
    if (part.startsWith('**') && part.endsWith('**')) {
      return <strong key={index}>{part.slice(2, -2)}</strong>
    }
    return <React.Fragment key={index}>{part}</React.Fragment>
  })
}

const Guide: React.FC = () => {
  const { token } = theme.useToken()
  const { guideMarkdown } = useMoziBuilder()
  const blocks = parseMarkdown(guideMarkdown)

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <Title level={4} style={{ margin: 0 }}>
          开发平台 / 操作手册
        </Title>
        <Text type="secondary">内容由宿主应用注入</Text>
      </div>

      <article
        style={{
          maxWidth: 960,
          lineHeight: 1.8,
          color: token.colorText,
        }}
      >
        {blocks.map((block, index) => {
          if (block.type === 'heading') {
            const level = Math.min(block.level + 1, 5) as 2 | 3 | 4 | 5
            return (
              <Title
                key={index}
                level={level}
                style={{
                  marginTop: index === 0 ? 0 : 32,
                  marginBottom: 12,
                }}
              >
                {block.text}
              </Title>
            )
          }

          if (block.type === 'paragraph') {
            return (
              <p key={index} style={{ margin: '10px 0' }}>
                {renderInline(block.text)}
              </p>
            )
          }

          if (block.type === 'unorderedList' || block.type === 'orderedList') {
            const ListTag = block.type === 'orderedList' ? 'ol' : 'ul'
            return (
              <ListTag key={index} style={{ margin: '10px 0 16px', paddingLeft: 24 }}>
                {block.items.map((item, itemIndex) => (
                  <li key={itemIndex}>{renderInline(item)}</li>
                ))}
              </ListTag>
            )
          }

          if (block.type === 'code') {
            return (
              <pre
                key={index}
                style={{
                  margin: '14px 0 18px',
                  padding: 16,
                  overflowX: 'auto',
                  borderRadius: token.borderRadius,
                  background: token.colorFillTertiary,
                  border: `1px solid ${token.colorBorderSecondary}`,
                }}
              >
                <code style={{ fontFamily: token.fontFamilyCode, fontSize: 13 }}>
                  {block.code}
                </code>
              </pre>
            )
          }

          return (
            <div key={index} style={{ overflowX: 'auto', margin: '14px 0 18px' }}>
              <table
                style={{
                  width: '100%',
                  borderCollapse: 'collapse',
                  border: `1px solid ${token.colorBorderSecondary}`,
                }}
              >
                <thead>
                  <tr>
                    {block.headers.map((header, headerIndex) => (
                      <th
                        key={headerIndex}
                        style={{
                          textAlign: 'left',
                          padding: '10px 12px',
                          background: token.colorFillQuaternary,
                          borderBottom: `1px solid ${token.colorBorderSecondary}`,
                        }}
                      >
                        {renderInline(header)}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {block.rows.map((row, rowIndex) => (
                    <tr key={rowIndex}>
                      {block.headers.map((_, cellIndex) => (
                        <td
                          key={cellIndex}
                          style={{
                            padding: '10px 12px',
                            borderTop: `1px solid ${token.colorBorderSecondary}`,
                          }}
                        >
                          {renderInline(row[cellIndex] || '')}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        })}
      </article>
    </div>
  )
}

export default Guide
