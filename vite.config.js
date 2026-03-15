import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: 'cmd/kompass/dist',
    emptyOutDir: true,
  },
  server: {
    host: true,
    port: 8081,
    strictPort: true,
    proxy: {
      '/graph': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/tree': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/healthz': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/readyz': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/stats': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
