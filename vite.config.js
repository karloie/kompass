import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { viteSingleFile } from 'vite-plugin-singlefile'

export default defineConfig({
  root: 'web',
  plugins: [vue(), viteSingleFile()],
  base: './',
  build: {
    outDir: '../pkg/tree/dist',
    emptyOutDir: true,
  },
  server: {
    host: true,
    port: 8081,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        proxyTimeout: 60000,
        timeout: 60000,
      },
    },
  },
})
