import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';
import { viteSingleFile } from 'vite-plugin-singlefile';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react(), tailwindcss(), viteSingleFile()],
  resolve: { alias: { '@': fileURLToPath(new URL('src', import.meta.url)) } },
  build: { outDir: 'dist', emptyOutDir: true, license: { fileName: 'licenses.json' } },
  test: { include: ['src/**/*.test.ts'] },
});
