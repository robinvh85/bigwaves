/// <reference types="vitest/config" />
import path from 'node:path'

import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const backend = process.env.BACKEND_URL ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [
    tanstackRouter({ target: 'react', autoCodeSplitting: true }),
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: { '@': path.resolve(import.meta.dirname, './src') },
  },
  server: {
    port: 5173,
    // Same origin as production: the browser only talks to Vite, which forwards API and auth
    // calls to the Go server, so the session cookie works unchanged.
    proxy: {
      '/api': { target: backend, changeOrigin: false },
      '/auth': { target: backend, changeOrigin: false },
    },
  },
  build: {
    rolldownOptions: {
      output: {
        // Long-lived vendor chunks: they change rarely, so browsers keep them cached across deploys.
        codeSplitting: {
          groups: [
            { name: 'react', test: /node_modules[\\/](react|react-dom|scheduler)[\\/]/ },
            { name: 'tanstack', test: /node_modules[\\/]@tanstack[\\/]/ },
            { name: 'radix', test: /node_modules[\\/](radix-ui|@radix-ui)[\\/]/ },
          ],
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
