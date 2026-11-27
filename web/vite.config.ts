import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/auth': 'http://localhost:8089',
      '/health': 'http://localhost:8089',
      '/items': 'http://localhost:8089',
      '/swagger': 'http://localhost:8089',
    },
  },
})
