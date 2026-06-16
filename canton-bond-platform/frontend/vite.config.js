import { defineConfig } from 'vite';

const apiTarget = process.env.BACKEND_HTTPS || process.env.BACKEND_HTTP;
if (!apiTarget) {
  throw new Error('BACKEND_HTTP not set. Run the app through Aspire.');
}

export default defineConfig({
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: apiTarget,
        changeOrigin: true,
      },
    },
  },
});
