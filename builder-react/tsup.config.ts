import { defineConfig } from 'tsup'

export default defineConfig({
  entry: ['index.ts'],
  format: ['esm'],
  dts: true,
  clean: true,
  external: [
    'react',
    'react-dom',
    'react-router-dom',
    'antd',
    '@ant-design/icons',
    '@xyflow/react',
    'axios',
    'dayjs',
    'elkjs',
    'zustand',
  ],
  injectStyle: true,
  sourcemap: true,
})
