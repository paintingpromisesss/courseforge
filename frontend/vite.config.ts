import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendPort = process.env.COURSEFORGE_PORT || 6770;
const backendHost = process.env.COURSEFORGE_HOST || '127.0.0.1';
const backendTarget = `http://${backendHost}:${backendPort}`;

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('@codemirror') || id.includes('/codemirror/')) {
            return 'codemirror';
          }
          if (id.includes('highlight.js') || id.includes('rehype-highlight') || id.includes('lowlight')) {
            return 'highlight';
          }
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': {
        target: backendTarget,
        timeout: 300000,
        proxyTimeout: 300000,
      },
      '/swagger': backendTarget,
    },
  },
})
