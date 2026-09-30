import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: Object.fromEntries(
      ['/api', '/uploads', '/sitemap.xml', '/robots.txt'].map((path) => [
        path,
        { target: 'http://localhost:8080', changeOrigin: false },
      ]),
    ),
  },
})
