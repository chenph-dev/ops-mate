import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
      '@wailsjs': path.resolve(__dirname, 'wailsjs'),
    },
  },
  build: {
    // 桌面应用把前端资源本地嵌入单个二进制、不走网络加载，
    // 因此 Vite 面向网页的 500kB 单 chunk 告警阈值不适用。
    // 直接调高阈值消除误报，无需做繁琐的手动分包逻辑。
    chunkSizeWarningLimit: 2000,
  },
});
