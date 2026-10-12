// Zooms and pans a diagram inside its frame, which first shows the whole diagram. The frame's size comes
// from CSS, and the viewBox is rewritten to the frame's aspect ratio, so what shows is the viewBox.
// messages holds the toolbar's wording by language; the lang attribute of the page's root picks one.
function zoomable(messages) {
  const MAX = 4;
  const lang = document.documentElement.lang || 'en';
  const msg = messages[lang] || messages[lang.split('-')[0]] || messages.en;
  const setup = (wrap) => {
    // A page may hold these assets more than once (fragments rendered apart); a second run does nothing
    if (wrap.dataset.zoomable) return;
    wrap.dataset.zoomable = 'on';
    const svg = wrap.querySelector('svg');
    const [, , W, H] = svg.getAttribute('viewBox').split(' ').map(Number);
    // s is screen px per diagram unit and (cx, cy) the centre of the view. While fitted, the whole
    // diagram stays shown when the frame resizes
    let s = 1, cx = W / 2, cy = H / 2, fitted = true;
    const fit = () => Math.min(wrap.clientWidth / W, wrap.clientHeight / H);
    // Zooming out stops at the whole diagram or the actual size, whichever is smaller
    const min = () => Math.min(fit(), 1);
    const tools = document.createElement('div');
    tools.className = 'dg-tools';
    const hint = document.createElement('span');
    hint.textContent = msg.diagram_zoom_hint;
    tools.append(hint);
    for (const [k, text, label] of [
      ['in', '+', msg.diagram_zoom_in], ['out', '\u2212', msg.diagram_zoom_out],
      ['fit', msg.diagram_zoom_fit, msg.diagram_zoom_fit_label], ['one', msg.diagram_zoom_one, msg.diagram_zoom_one_label],
    ]) {
      const b = document.createElement('button');
      b.type = 'button';
      b.dataset.zoom = k;
      b.title = label;
      b.setAttribute('aria-label', label);
      b.textContent = text;
      tools.append(b);
    }
    wrap.before(tools);
    const button = (k) => tools.querySelector(`[data-zoom="${k}"]`);

    const render = () => {
      const w = wrap.clientWidth, h = wrap.clientHeight;
      if (!w || !h) return;
      if (fitted) [s, cx, cy] = [fit(), W / 2, H / 2];
      s = Math.min(Math.max(s, min()), MAX);
      const vw = w / s, vh = h / s;
      // Centre a direction the view is wider than the diagram in; otherwise keep the view inside it
      const clamp = (c, v, size) => (v >= size ? size / 2 : Math.min(Math.max(c, v / 2), size - v / 2));
      cx = clamp(cx, vw, W);
      cy = clamp(cy, vh, H);
      svg.setAttribute('viewBox', `${cx - vw / 2} ${cy - vh / 2} ${vw} ${vh}`);
      wrap.classList.toggle('zoomed', vw < W - 0.5 || vh < H - 0.5);
      button('in').disabled = s >= MAX;
      button('out').disabled = s <= min();
      button('fit').disabled = fitted;
      button('one').disabled = Math.abs(s - 1) < 1e-3;
    };
    // Screen coordinates as px from the top left inside the frame's border
    const local = (x, y) => {
      const r = wrap.getBoundingClientRect();
      return [x - r.left - wrap.clientLeft, y - r.top - wrap.clientTop];
    };
    // Scales by k about (ax, ay), in local coordinates, keeping the diagram point under it in place
    const zoomAt = (k, ax = wrap.clientWidth / 2, ay = wrap.clientHeight / 2) => {
      const ns = Math.min(Math.max(s * k, min()), MAX);
      const dx = ax - wrap.clientWidth / 2, dy = ay - wrap.clientHeight / 2;
      cx += dx / s - dx / ns;
      cy += dy / s - dy / ns;
      s = ns;
      fitted = false;
      render();
    };

    tools.addEventListener('click', (e) => {
      const k = e.target.closest('button')?.dataset.zoom;
      if (k === 'in') zoomAt(1.25);
      if (k === 'out') zoomAt(0.8);
      if (k === 'one') zoomAt(1 / s);
      if (k === 'fit') {
        fitted = true;
        render();
      }
    });
    // A plain wheel scrolls the page. A trackpad pinch arrives as a wheel with ctrlKey too
    wrap.addEventListener('wheel', (e) => {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      const dy = e.deltaMode === 1 ? e.deltaY * 16 : e.deltaY;
      zoomAt(Math.exp(-dy * 0.002), ...local(e.clientX, e.clientY));
    }, { passive: false });

    // One finger or a mouse moves the diagram; two fingers zoom about their midpoint while moving it
    const points = new Map();
    const gesture = () => {
      const [a, b = a] = [...points.values()];
      return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2, d: Math.hypot(a.x - b.x, a.y - b.y) };
    };
    wrap.addEventListener('pointerdown', (e) => {
      // While the whole diagram shows, a mouse is left to select text
      if (e.button !== 0 || (e.pointerType === 'mouse' && !wrap.classList.contains('zoomed'))) return;
      e.preventDefault();
      wrap.setPointerCapture(e.pointerId);
      points.set(e.pointerId, { x: e.clientX, y: e.clientY });
    });
    wrap.addEventListener('pointermove', (e) => {
      if (!points.has(e.pointerId)) return;
      const before = gesture();
      points.set(e.pointerId, { x: e.clientX, y: e.clientY });
      const after = gesture();
      cx -= (after.x - before.x) / s;
      cy -= (after.y - before.y) / s;
      if (before.d && after.d) zoomAt(after.d / before.d, ...local(after.x, after.y));
      else render();
    });
    for (const type of ['pointerup', 'pointercancel']) wrap.addEventListener(type, (e) => points.delete(e.pointerId));
    // The first resize notice comes just before painting, so settle the state once now for code
    // (tests) that reads it without waiting for a paint
    render();
    new ResizeObserver(render).observe(wrap);
  };
  const init = () => document.querySelectorAll('.dg-wrap').forEach(setup);
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
}
