import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          // Chart.js + zoom plugin are the bulk of the bundle; splitting them
          // out keeps the app chunk small and lets the vendor chunk cache
          // across app-only changes.
          charts: ['chart.js', 'chartjs-plugin-zoom'],
        },
      },
    },
  },
})
