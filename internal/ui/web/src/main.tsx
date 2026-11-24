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
