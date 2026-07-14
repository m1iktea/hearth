import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { mockApi } from './mock/api.js'

export default defineConfig(({ mode }) => ({
  plugins: [vue(), ...(mode === 'mock' ? [mockApi()] : [])],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
}))
