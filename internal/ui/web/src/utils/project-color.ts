// projectColor is one of six colours, by the project's place in projectOrder; the faint one for none, and for one no
// entry has any more (picked, its last entry gone)
// LIMIT: from the seventh project on the colours repeat; let the viewer pick them if projects come to be that many
export const projectColor = (name: string, ranks: ReadonlyMap<string, number>): string => {
  const rank = ranks.get(name);
  return rank === undefined ? 'var(--color-faint)' : `var(--color-pc${rank % 6})`;
};
