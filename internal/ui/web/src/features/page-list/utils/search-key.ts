// searchKey is what a key in the search field does: Escape empties it and leaves it; Enter, where the field goes
// somewhere with its words (go), goes there
export function searchKey(
  key: string,
  words: string,
  to: { go?: ((words: string) => void) | undefined; leave: () => void },
) {
  if (key === 'Enter') to.go?.(words);
  if (key === 'Escape') to.leave();
}
