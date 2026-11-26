function drop(e: Event): void {
  e.preventDefault();
  e.stopPropagation();
}
function dropPress(e: MouseEvent): void {
  if (e.detail > 1) drop(e);
  else if (e.type === 'mousedown') {
    document.removeEventListener('mousedown', dropPress, true);
    document.removeEventListener('click', dropPress, true);
  }
}
function dropKey(e: KeyboardEvent): void {
  if (e.repeat) drop(e);
  else newKey();
}

// newKey tells of a key begun where this document is not told of it, in an iframe: its repeats are not dropped
export function newKey(): void {
  document.removeEventListener('keydown', dropKey, true);
}

// dropRest drops what is left of the press or key a modal opens or closes by, which would reach what is then on top: the
// second of a double press, until a first one, and the repeats of a key held, until another key. Asked twice, the
// listeners are added once
// LIMIT: pointerdown, pointerup, keyup and dblclick are not dropped, so what acts on them is reached; a first press Chrome
// counts with the one before is dropped
// LIMIT: a press in an iframe is not seen, nor a key there that its page does not hand over (newKey). A modal that opens
// or closes with no press or key (a write's answer, a message) has the listeners stay till the next: lost only if the
// first is one Chrome counts with the one before, or a repeat
export function dropRest(): void {
  document.addEventListener('mousedown', dropPress, true);
  document.addEventListener('click', dropPress, true);
  document.addEventListener('keydown', dropKey, true);
}
