// Scrolls one of the two pages of a diff side by side with the other, by the blocks of both versions that go together
// (the class floe-diff-pair-<n> on both): it tells the frame which block is at the top of the page and how far into it,
// and goes where the frame tells it the other is. A page moved so tells nothing until the person moves it, so that the
// two never move each other back and forth. What a page could shadow is read through window.
function diffsync() {
  const here = window.location.protocol + '//' + window.location.host;
  const PAIR = /(?:^|\s)floe-diff-pair-(\d+)(?=\s|$)/;
  let moved = false;
  let asked = false;
  // The first block of a pair whose bottom is below the top of the page, skipping those hidden (in a tab not picked)
  const top = () => {
    for (const e of window.document.querySelectorAll('[class*="floe-diff-pair-"]')) {
      if (e.getClientRects().length === 0) continue;
      const m = PAIR.exec(e.getAttribute('class') ?? '');
      const r = e.getBoundingClientRect();
      if (!m || r.height <= 0 || r.bottom <= 0) continue;
      return { pair: Number(m[1]), off: Math.min(Math.max(-r.top / r.height, 0), 1) };
    }
    return null;
  };
  // Once a frame at most, however many scrolls come in it
  const tell = () => {
    if (moved || asked) return;
    asked = true;
    window.requestAnimationFrame(() => {
      asked = false;
      const at = top();
      if (at && !moved) window.parent.postMessage({ artifactDiffAt: at }, here);
    });
  };
  window.addEventListener('scroll', tell, { passive: true });
  for (const name of ['wheel', 'touchstart', 'keydown', 'pointerdown']) {
    window.addEventListener(name, () => {
      moved = false;
    }, { capture: true, passive: true });
  }
  window.addEventListener('message', (e) => {
    if (e.source !== window.parent || e.origin !== here) return;
    const go = e.data?.artifactDiffGo;
    if (!go || !Number.isInteger(go.pair) || go.pair < 0 || !(go.off >= 0 && go.off <= 1)) return;
    const el = window.document.getElementsByClassName('floe-diff-pair-' + go.pair)[0];
    if (!el) return;
    moved = true;
    const r = el.getBoundingClientRect();
    window.scrollBy(0, r.top + go.off * r.height);
  });
}
