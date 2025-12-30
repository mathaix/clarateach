import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

const PORTAL_PORT = process.env.PORTAL_PORT ?? '3000';
const FRONTEND_PORT = parseInt(process.env.FRONTEND_PORT ?? '5173', 10);

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: FRONTEND_PORT,
    proxy: {
      '/api': {
        target: `http://localhost:8080`,
        changeOrigin: true,
      },
      '/debug/proxy': {
        target: `http://localhost:8080`,
        changeOrigin: true,
        ws: true,
      },
    },
  },
});
