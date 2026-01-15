import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte()],
  
  // Production: serve under /dashboard/
  base: '/dashboard/',
  
  // Dev server config
  server: {
    port: 5173,
    proxy: {
      // Proxy API requests to Go server
      '/autoctx/api': {
        target: 'http://localhost:11435',
        changeOrigin: true,
      },
      // Proxy health endpoints
      '/healthz': {
        target: 'http://localhost:11435',
        changeOrigin: true,
      },
      // Proxy metrics endpoint
      '/metrics': {
        target: 'http://localhost:11435',
        changeOrigin: true,
      },
      // Proxy SSE events (for future use)
      '/events': {
        target: 'http://localhost:11435',
        changeOrigin: true,
      },
    },
  },
  
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      // Exclude server-side modules from the client bundle
      external: [/^node:/],
    },
  },
  
  // Force client-side resolution for svelte
  resolve: {
    conditions: ['browser', 'import', 'module', 'default'],
  },

  // Optimize dependencies
  optimizeDeps: {
    include: ['svelte'],
    exclude: ['svelte/internal/server'],
  },
})
