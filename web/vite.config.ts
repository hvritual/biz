import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
const backend = process.env.BIZ_DEV_API_TARGET ?? 'http://127.0.0.1:8080'
const proxy = {
  '/api': { target: backend, changeOrigin: false, rewrite: (path: string) => path.replace(/^\/api/, '') },
  '/auth': { target: backend, changeOrigin: false },
}
export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: { host: '0.0.0.0', port: 5173, strictPort: true, proxy },
  preview: { host: '0.0.0.0', port: 4173, strictPort: true, proxy },
  build: {
    target: 'es2022',
    sourcemap: false,
    rollupOptions: { output: { manualChunks: { vue: ['vue', 'vue-router', 'pinia'] } } },
  },
})
