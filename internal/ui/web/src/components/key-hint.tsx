// The look of a key named in a line of words
export const KBD =
  'rounded-small border border-line bg-subtle px-1.5 py-0.75 font-mono text-label leading-none text-muted';

// KeyHint is a search modal's line of a key and what it does: gone where the window is narrow, and kept in its place,
// unseen, where there is no row for the keys to act on (shown false), so that the line keeps its height
export function KeyHint({
  keys,
  words,
  shown,
}: {
  readonly keys: readonly string[];
  readonly words: string;
  readonly shown: boolean;
}) {
  return (
    <span
      data-testid="k"
      className={`inline-flex items-center gap-1 whitespace-nowrap narrow:hidden ${shown ? '' : 'invisible'}`}
    >
      {keys.map((k) => (
        <kbd key={k} className={KBD}>
          {k}
        </kbd>
      ))}{' '}
      {words}
    </span>
  );
}
