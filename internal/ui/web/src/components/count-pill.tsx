// CountPill is how many a heading heads, in a pill dark enough to stand on either ground of the page, but not the ink of
// what is on or picked (the user's choice)
export function CountPill({ n }: { readonly n: number }) {
  return (
    <span data-testid="count" className="rounded-full border border-muted bg-muted px-[0.3rem] text-label text-on-ink">
      {n}
    </span>
  );
}
