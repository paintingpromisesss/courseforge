import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        timeout: 300000,
        proxyTimeout: 300000,
      },
      '/swagger': 'http://127.0.0.1:8080',
    },
  },
})
