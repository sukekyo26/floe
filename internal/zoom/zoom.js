// Zooms and pans what a frame holds, which first shows it whole: a diagram's svg, whose viewBox is rewritten to the
// frame's aspect ratio, or an img, which keeps the place and size the page gives it and is moved by a transform.
// words holds the wording by language, which the lang attribute of the page's root picks; frame and tools are the
// classes of the frame and the toolbar.
function zoomable({ words, frame: frameClass, tools: toolsClass }) {
  const MAX = 4;
  // A tag is matched as floe matches it: in lower case, with _ read as -
  const lang = (document.documentElement.lang || 'en').toLowerCase().replace(/_/g, '-');
  const msg = words[lang] || words[lang.split('-')[0]] || words.en;
  const setup = (wrap) => {
    // A page may hold this script more than once (a part each, fragments rendered apart); a frame another run set up,
    // or waits on an image for, is left alone
    if (wrap.dataset.zoomable) return;
    // The frame's first element is what it zooms: an svg or an img
    const first = wrap.firstElementChild;
    const svg = first?.localName === 'svg' ? first : null;
    const img = first?.localName === 'img' ? first : null;
    if (!svg && !img) return;
    if (img && !img.complete) {
      // Marked before waiting, so that the other runs do not wait too
      wrap.dataset.zoomable = 'pending';
      const again = () => {
        delete wrap.dataset.zoomable;
        setup(wrap);
      };
      img.addEventListener('load', again, { once: true });
      img.addEventListener('error', again, { once: true });
      return;
    }
    const [W, H] = svg ? (svg.getAttribute('viewBox') ?? '').split(/[\s,]+/).map(Number).slice(2) : [img.naturalWidth, img.naturalHeight];
    // A frame without a size to zoom (a broken image) is left alone, so that the frames after it still get theirs
    if (!(W > 0 && H > 0)) return;
    wrap.dataset.zoomable = 'on';
    // A browser drags an image away as a picture of it, which ends a move with pointercancel
    if (img) img.draggable = false;
    // s is screen px per unit (a diagram's, an image's pixel) and (cx, cy) the centre of the view. While fitted,
    // the whole shows when the frame resizes
    let s = 1, cx = W / 2, cy = H / 2, fitted = true;
    // While printing, the whole shows and the zoom is kept, to come back to after
    let printing = false;
    // Whether it may be zoomed: zoomed, or drawn smaller than its actual size. undefined until first rendered
    let offered;
    // The whole: a diagram fitted to the frame, an image as the page lays it out (its frame holds its frame line too)
    const fit = () => (svg ? Math.min(wrap.clientWidth / W, wrap.clientHeight / H) : img.clientWidth / W);
    // Zooming out stops at the whole or the actual size, whichever is smaller
    const min = () => Math.min(fit(), 1);
    // Drawn smaller than its actual size. A diagram's frame keeps its aspect ratio with its line, which takes 2 px out
    // of what the diagram is drawn in: one as wide as the column is not shrunk by that
    const shrunk = () => (svg ? Math.min(wrap.offsetWidth / W, wrap.offsetHeight / H) : fit()) < 1;
    const tools = document.createElement('div');
    tools.className = toolsClass;
    tools.hidden = true;
    for (const [k, text, label] of [
      ['in', '+', msg.zoom_in], ['out', '−', msg.zoom_out],
      ['fit', msg.zoom_fit, msg.zoom_fit_label], ['one', msg.zoom_one, msg.zoom_one_label],
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
    // How it is used: its tooltip for a mouse, and for the keyboard, which shows no tooltip, a line under the frame while
    // it has the focus (part.css), which is also its description. What the frame was written with stays.
    // LIMIT: a touch screen shows neither, at any width. A title of an element in the frame wins over the frame's in the
    // tooltip; the line says the same. Show the line on touch (pointer: coarse) if readers there come to miss it.
    // LIMIT: the line is the frame's next element, which a part of one's own that lays a frame out in a grid or a flex, or
    // writes .floe-zoom + x, has to allow for (docs/agents/parts.md); a frame in a box of its own holds it. Move it in the toolbar
    // if a part of floe's comes to do so.
    const title = wrap.hasAttribute('title'), described = wrap.hasAttribute('aria-describedby');
    const hint = document.createElement('div');
    hint.className = `${frameClass}-hint`;
    hint.id = `${frameClass}-hint-${document.querySelectorAll(`.${hint.className}`).length}`;
    hint.textContent = msg.zoom_hint;
    wrap.after(hint);

    // The toolbar, the tab stop, the frame's role, name, description and tooltip and touch-action go with whether it may be zoomed. Focus
    // in a toolbar that hides moves to the frame, which keeps -1 to hold it, not to the page's start
    const offer = (can) => {
      if (can === offered) return;
      offered = can;
      const focused = wrap.contains(document.activeElement) || tools.contains(document.activeElement);
      tools.hidden = !can;
      wrap.classList.toggle('can-zoom', can);
      wrap.tabIndex = can ? 0 : -1;
      if (can) {
        wrap.setAttribute('role', 'group');
        wrap.setAttribute('aria-label', msg.zoom_frame);
        if (!described) wrap.setAttribute('aria-describedby', hint.id);
        if (!title) wrap.title = msg.zoom_hint;
      } else {
        wrap.removeAttribute('role');
        wrap.removeAttribute('aria-label');
        if (!described) wrap.removeAttribute('aria-describedby');
        if (!title) wrap.removeAttribute('title');
        if (focused) wrap.focus();
      }
    };
    const show = (x, y, w, h) => {
      if (svg) {
        svg.setAttribute('viewBox', `${x} ${y} ${w} ${h}`);
        return;
      }
      if (fitted || printing) {
        img.style.transform = '';
        img.style.borderColor = '';
        return;
      }
      // The image lies at the whole's size, its pixels from its frame line in; scaled by k from its corner, a pixel
      // (px, py) is put where the view shows it, s px a pixel from (x, y), as the diagram's viewBox puts it
      const k = s / fit(), ox = img.clientLeft, oy = img.clientTop;
      // Its line, scaled, would thicken: zoomed, it is dropped, over whatever layer of CSS draws it
      img.style.borderColor = 'transparent';
      img.style.transform = `translate(${ox * (1 - k) - x * s}px, ${oy * (1 - k) - y * s}px) scale(${k})`;
    };

    const render = () => {
      // Before the size: a diagram in a hidden tab has none at beforeprint, and prints all the same
      if (printing) {
        show(0, 0, W, H);
        wrap.classList.remove('zoomed');
        return;
      }
      const w = wrap.clientWidth, h = wrap.clientHeight;
      if (!w || !h) return;
      if (fitted) [s, cx, cy] = [fit(), W / 2, H / 2];
      s = Math.min(Math.max(s, min()), MAX);
      const vw = w / s, vh = h / s;
      // A direction the view is wider than what it shows in: a diagram is centred, an image kept at its top left,
      // where the page lays it. Otherwise the view is kept inside it
      const clamp = (c, v, size) => (v >= size ? (svg ? size / 2 : v / 2) : Math.min(Math.max(c, v / 2), size - v / 2));
      cx = clamp(cx, vw, W);
      cy = clamp(cy, vh, H);
      show(cx - vw / 2, cy - vh / 2, vw, vh);
      wrap.classList.toggle('zoomed', vw < W - 0.5 || vh < H - 0.5);
      offer(!fitted || shrunk());
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
    // Scales by k about (ax, ay), in local coordinates, keeping the point under it in place
    const zoomAt = (k, ax = wrap.clientWidth / 2, ay = wrap.clientHeight / 2) => {
      const ns = Math.min(Math.max(s * k, min()), MAX);
      const dx = ax - wrap.clientWidth / 2, dy = ay - wrap.clientHeight / 2;
      cx += dx / s - dx / ns;
      cy += dy / s - dy / ns;
      s = ns;
      // Zooming out as far as the whole is fitting again: it follows the frame's size from then on
      fitted = Math.abs(ns / fit() - 1) < 1e-9;
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
      if (!offered || (!e.ctrlKey && !e.metaKey)) return;
      e.preventDefault();
      // deltaMode 1 counts lines and 2 pages; a page is the frame's height
      const dy = e.deltaY * [1, 16, wrap.clientHeight][e.deltaMode];
      zoomAt(Math.exp(-dy * 0.002), ...local(e.clientX, e.clientY));
    }, { passive: false });

    // One finger or a mouse moves it; two fingers zoom about their midpoint while moving it
    const points = new Map();
    const gesture = () => {
      const [a, b = a] = [...points.values()];
      return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2, d: Math.hypot(a.x - b.x, a.y - b.y) };
    };
    wrap.addEventListener('pointerdown', (e) => {
      // While the whole shows, a mouse is left to select text
      if (!offered || e.button !== 0 || (e.pointerType === 'mouse' && !wrap.classList.contains('zoomed'))) return;
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
    for (const type of ['pointerup', 'pointercancel', 'lostpointercapture']) wrap.addEventListener(type, (e) => points.delete(e.pointerId));

    // The keyboard reaches the frame too: + (or =), - and 0 zoom in, out and to the whole, as the buttons do, and
    // the arrow keys move a zoomed view by 40 screen px. Keys with Ctrl, ⌘ or Alt are the browser's (its zoom,
    // back); Shift is not looked at, as + is Shift and = on many keyboards
    wrap.addEventListener('keydown', (e) => {
      if (!offered || e.ctrlKey || e.metaKey || e.altKey) return;
      const zoom = { '+': 'in', '=': 'in', '-': 'out', 0: 'fit' }[e.key];
      if (zoom) {
        e.preventDefault();
        button(zoom).click();
        return;
      }
      const step = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] }[e.key];
      if (!step || !wrap.classList.contains('zoomed')) return;
      e.preventDefault();
      cx += (step[0] * 40) / s;
      cy += (step[1] * 40) / s;
      render();
    });
    // The first resize notice comes just before painting, so settle the state once now for code
    // (tests) that reads it without waiting for a paint
    render();
    new ResizeObserver(render).observe(wrap);
    addEventListener('beforeprint', () => {
      printing = true;
      render();
    });
    addEventListener('afterprint', () => {
      printing = false;
      render();
    });
  };
  // LIMIT: a frame in a shadow root is not found, and gets no toolbar or zoom; look in the shadow roots if one is asked for
  const init = () => document.querySelectorAll('.' + frameClass).forEach(setup);
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
}
