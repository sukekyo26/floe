import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { viteSingleFile } from 'vite-plugin-singlefile';
import type { Plugin } from 'vite';
import { defineConfig } from 'vitest/config';

// A page's sandbox, as the server sends it (internal/server/pages.go pageCSP; TestGalleryPageSandbox holds the two the
// same)
const PAGE_CSP =
  'sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox allow-forms allow-modals allow-downloads';

// galleryPage answers the gallery's dev server for a page (/a/…) with gallery/page.html, as the server sends a page: in
// the sandbox, of origin null, and whole, past Vite's changes to HTML, whose scripts that page could not load. It
// serves alone: no build has it
const galleryPage: Plugin = {
  name: 'gallery-page',
  apply: 'serve',
  configureServer(server) {
    server.middlewares.use('/a/', (_req, res) => {
      res.setHeader('Content-Type', 'text/html; charset=utf-8');
      res.setHeader('Content-Security-Policy', PAGE_CSP);
      res.end(readFileSync(new URL('gallery/page.html', import.meta.url)));
    });
  },
};

export default defineConfig({
  plugins: [react(), tailwindcss(), viteSingleFile(), galleryPage],
  resolve: { alias: { '@': fileURLToPath(new URL('src', import.meta.url)) } },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    modulePreload: { polyfill: false },
    license: { fileName: 'licenses.json' },
  },
  test: { include: ['src/**/*.test.{ts,tsx}'] },
});
