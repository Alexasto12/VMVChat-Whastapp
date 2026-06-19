import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// El build sale directo a backend/dist para que Go lo embeba con go:embed.
export default defineConfig({
  plugins: [svelte()],
  build: { outDir: '../backend/dist', emptyOutDir: true },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/send': 'http://localhost:8080',
      '/webhook': 'http://localhost:8080',
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
})
