import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/transactions': 'http://localhost:8080',
      '/upload': 'http://localhost:8080',
      '/files': 'http://localhost:8080',
      '/calculate': 'http://localhost:8080',
    },
  },
})
