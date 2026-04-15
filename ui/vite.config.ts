import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';

// The Go demo server listens on 127.0.0.1:8787 by default (see
// cmd/slogview-demo). Vite serves the UI on :5173 and proxies /api
// requests to the Go backend so the browser talks to a single origin.
export default defineConfig({
  plugins: [preact()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8787',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
