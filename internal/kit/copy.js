// A button in the part this script is in, that copies the text of its elements marked data-floe-copy, each ended
// by a line break. words are its words in the page's language: copy, copied, failed.
function copyButton(words) {
  const part = document.currentScript.closest('[data-floe-part]');
  const icon = (d) =>
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" ' +
    `stroke-linejoin="round" aria-hidden="true"><path d="${d}"/></svg>`;
  const button = document.createElement('button');
  button.type = 'button';
  button.className = 'floe-copy';
  button.innerHTML = icon('M9 9h10v11H9zM5 15V4h10') + icon('m5 12.5 4.5 4.5L19 7.5');
  const status = document.createElement('span');
  status.className = 'floe-copy-status';
  status.setAttribute('role', 'status');
  let timer;
  // The press whose result shows: one pressed again before its copy ends is dropped
  let presses = 0;
  const show = (label, copied) => {
    button.classList.toggle('copied', copied);
    button.title = label;
    button.setAttribute('aria-label', label);
  };
  const text = () =>
    [...part.querySelectorAll('[data-floe-copy]')]
      .map((e) => (e.textContent.endsWith('\n') ? e.textContent : e.textContent + '\n'))
      .join('');
  // The page's frame may not be let write the clipboard (VS Code's Simple Browser), where the copy command still
  // writes; its event carries the text, so neither the focus nor the selection moves
  const command = (s) => {
    const put = (e) => {
      e.clipboardData.setData('text/plain', s);
      e.preventDefault();
    };
    document.addEventListener('copy', put);
    try {
      return document.execCommand('copy');
    } catch {
      return false;
    } finally {
      document.removeEventListener('copy', put);
    }
  };
  button.addEventListener('click', async () => {
    const press = ++presses;
    clearTimeout(timer);
    show(words.copy, false);
    status.textContent = '';
    const s = text();
    // navigator.clipboard is missing where the page is not a secure context: that throws, not rejects
    let ok;
    try {
      await navigator.clipboard.writeText(s);
      ok = true;
    } catch {
      ok = command(s);
    }
    if (press !== presses) return;
    const label = ok ? words.copied : words.failed;
    show(label, ok);
    // In a later task than the emptying, so that the same words are read out again
    setTimeout(() => {
      status.textContent = label;
    }, 0);
    timer = setTimeout(() => {
      show(words.copy, false);
      status.textContent = '';
    }, 1500);
  });
  show(words.copy, false);
  document.currentScript.after(button, status);
}
