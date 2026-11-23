// The comments of web/src that name ui.html, a line of it or a function of it (ui.html 1234, ui.html openFind) mean
// internal/ui/ui.html at 80b43ab0, the old management page these screens were rebuilt from: git show 80b43ab0:internal/ui/ui.html
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import './index.css';
import { App } from '@/app/app';
import { startFrame } from '@/app/start';
import { getBoot, readBoot, setBoot } from '@/config/boot';

const root = document.getElementById('root');
if (root !== null) {
  try {
    // Read before anything is drawn: no module reads the boot values as it loads
    setBoot(readBoot(document.getElementById('boot')?.textContent));
    if (getBoot().screen === 'frame') startFrame();
    createRoot(root).render(
      <StrictMode>
        <App search={window.location.search} />
      </StrictMode>,
    );
  } catch (err) {
    root.textContent = err instanceof Error ? err.message : String(err);
  }
}
