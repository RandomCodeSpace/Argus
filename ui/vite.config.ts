import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5174,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
      '/mcp': 'http://localhost:8080',
      '/metrics': 'http://localhost:8080',
    },
  },
  build: {
    outDir: '../internal/ui/dist',
    emptyOutDir: true,
  },
})
