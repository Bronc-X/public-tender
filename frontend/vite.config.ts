import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [tailwindcss(), react()],
  server: {
    host: '127.0.0.1',
    proxy: {
      '/api': {
        target: 'http://localhost:8081',
        changeOrigin: true,
        timeout: 600_000,
      },
      '/files': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
  build: {
    // P1-2: 优化代码分割策略
    rollupOptions: {
      output: {
        manualChunks: (id: string) => {
          // 分离 React 核心
          if (id.includes('node_modules/react')) {
            return 'react-vendor';
          }
          // 分离 antd 组件库（按功能分组）
          if (id.includes('node_modules/antd')) {
            if (id.includes('typography')) return 'antd-typography';
            if (id.includes('form')) return 'antd-form';
            if (id.includes('table')) return 'antd-table';
            if (id.includes('modal')) return 'antd-modal';
            if (id.includes('select')) return 'antd-select';
            return 'antd-vendor';
          }
          // 分离图表库（如果有）
          if (id.includes('node_modules/@ant-design/charts') || id.includes('node_modules/@antv/')) {
            return 'charts-vendor';
          }
          // 分离路由库
          if (id.includes('node_modules/react-router')) {
            return 'router-vendor';
          }
          // 分离 axios
          if (id.includes('node_modules/axios')) {
            return 'axios-vendor';
          }
          // 分离 markdown 处理
          if (id.includes('node_modules/react-markdown') || id.includes('node_modules/remark-')) {
            return 'markdown-vendor';
          }
        },
      },
    },
    // 提高 chunk 大小警告阈值，因为我们已经做了分割
    chunkSizeWarningLimit: 600,
  },
})
