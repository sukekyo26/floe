// The place a page is read at, in the contents just before this script: of its links to a part of this page, the last
// whose target has gone over a line 3 tenths down the window, the first at the page's top and the last at its end, or
// the one pressed while its target is in sight (the last targets of a page may never reach the line). It is marked aria-current and kept in sight where the contents scroll.
// The script stands before what the links go to, so it finds them once the page is read.
// LIMIT: the targets are measured at each scroll and resize; a page of hundreds of headings has not been timed.
function contents() {
  const nav = document.currentScript.previousElementSibling;
  const links = [...nav.querySelectorAll('a')].filter((a) => (a.getAttribute('href') || '').startsWith('#'));
  let spots = [];
  let pressed = null;
  const mark = () => {
    let at = -1;
    spots.forEach((spot, i) => {
      if (spot && spot.getBoundingClientRect().top <= innerHeight * 0.3) at = i;
    });
    if (scrollY <= 0) at = 0;
    else if (innerHeight + scrollY >= document.documentElement.scrollHeight - 2) at = spots.length - 1;
    const sight = pressed !== null && spots[pressed] && spots[pressed].getBoundingClientRect();
    if (sight && sight.top >= -1 && sight.top < innerHeight) at = pressed;
    else pressed = null;
    links.forEach((a, i) => (i === Math.max(at, 0) ? a.setAttribute('aria-current', 'location') : a.removeAttribute('aria-current')));
    const link = links[Math.max(at, 0)];
    if (link && nav.scrollHeight > nav.clientHeight) {
      const box = nav.getBoundingClientRect();
      const r = link.getBoundingClientRect();
      if (r.top < box.top + 24 || r.bottom > box.bottom - 24) nav.scrollTop += r.top - box.top - box.height / 2;
    }
  };
  nav.addEventListener('click', (e) => {
    pressed = links.indexOf(e.target.closest('a'));
    if (pressed < 0) pressed = null;
  });
  addEventListener('DOMContentLoaded', () => {
    // The href is the id as written (getAttribute), so no %-decoding, which a broken % would throw on
    spots = links.map((a) => document.getElementById(a.getAttribute('href').slice(1)));
    mark();
  });
  addEventListener('scroll', mark, { passive: true });
  addEventListener('hashchange', mark);
  addEventListener('resize', mark);
}
