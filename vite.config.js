import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  root: 'client',
  build: {
    outDir: '../static',
  },
  // base: '/ui',
  server: {
    proxy: {
      '/cosmos/api': {
        target: 'https://localhost:8443',
        secure: false,
      }
    }
  }
})
