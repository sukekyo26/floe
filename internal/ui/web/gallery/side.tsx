// The tree on the frame's left of a folder's page alone, in a window of its own: the tree, its edge and what opens it
// stand fixed on the window and its width goes to the root. A fake server answers the folder's directories; the tree's
// width is kept in memory, never in the browser's storage. ?gone shows a folder not found
import './gallery.css';

import { StrictMode, useState } from 'react';
import { createRoot } from 'react-dom/client';

import { setBoot, type Boot } from '@/config/boot';
import { SideTree } from '@/features/page-tree/components/side-tree';
import type { Client } from '@/lib/api-client';
import { createStore } from '@/lib/store';
import { createKeptSide } from '@/stores/kept-side';
import { createFolders } from '@/stores/open-folders';
import type { Listing, Page } from '@/types/page';

const ago = new Date(Date.now() - 3600000).toISOString();
const files = (...names: string[]) => names.map((file) => ({ file, updatedAt: ago }));
// The folder's directories, as the server would list them
const DIRS: Record<string, Listing> = {
  '': { dirs: ['api', 'guide'], files: files('index.html'), more: 0 },
  api: { dirs: ['many'], files: files('api/a.html'), more: 0 },
  'api/many': {
    dirs: [],
    files: files(...Array.from({ length: 80 }, (_, i) => `api/many/p${String(i).padStart(3, '0')}.html`)),
    more: 0,
  },
  guide: { dirs: ['deep', 'empty'], files: files('guide/setup.html', 'guide/start.html'), more: 0 },
  'guide/deep': { dirs: [], files: files('guide/deep/x.html'), more: 0 },
  'guide/empty': { dirs: [], files: [], more: 0 },
};
const client = {
  api: (url: string) => {
    const dir = new URL(url, window.location.href).searchParams.get('dir') ?? '';
    const l = DIRS[dir];
    return new Promise<Response>((answer) => {
      setTimeout(() => {
        answer(l ? new Response(JSON.stringify(l)) : new Response('', { status: 404 }));
      }, 150);
    });
  },
  noWrites: () => null,
  writesSent: () => 0,
  reached: createStore(true),
} satisfies Client;

const held = new Map<string, string>();
const side = createKeptSide('frame.side', 280, {
  storage: { read: (k) => held.get(k) ?? null, write: (k, v) => held.set(k, v), remove: (k) => held.delete(k) },
});
const folders = createFolders({ client });
const params = new URLSearchParams(window.location.search);
const file = params.get('file') ?? 'guide/setup.html';
const docs: Page = {
  slug: 'docs',
  title: 'Docs',
  folder: true,
  description: '',
  createdAt: ago,
  updatedAt: ago,
  taskProject: '/',
};

function Frame() {
  const [found, setFound] = useState(!params.has('gone'));
  return (
    <div className="flex h-dvh flex-col">
      <header className="flex h-bar flex-none items-center gap-3 border-b border-line bg-panel px-4 text-small">
        <strong>The frame’s tree</strong>
        <a href="?file=guide%2Fsetup.html">guide/setup.html</a>
        <a href="?file=api%2Fmany%2Fp079.html">the last of 80</a>
        <button
          type="button"
          onClick={() => {
            setFound(!found);
          }}
        >
          {found ? 'The folder gone' : 'The folder back'}
        </button>
        <span className="text-muted">Narrow under 40rem: over the page, kept nowhere</span>
      </header>
      <main className="ms-(--side,0px) min-h-0 flex-1 overflow-auto p-6">
        <p>
          The page framed: <code>{file}</code>. Kept: <code>{held.get('frame.side') ?? 'nothing'}</code> (as drawn last)
        </p>
      </main>
      <SideTree folder={found ? docs : null} file={file} folders={folders} side={side} />
    </div>
  );
}

const el = document.getElementById('root');
const boot = import.meta.glob<Boot>('./boot.json', { eager: true, import: 'default' })['./boot.json'];
if (el !== null && boot) {
  setBoot(boot);
  // The tree's first read, as the page's read of the list makes it
  void folders.readSide('docs', file, AbortSignal.timeout(5000)).then((take) => take?.());
  createRoot(el).render(
    <StrictMode>
      <Frame />
    </StrictMode>,
  );
} else if (el !== null) {
  el.textContent = 'gallery/boot.json is not there: start the gallery with just web-gallery';
}
