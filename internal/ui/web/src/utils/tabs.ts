// nextTab is the tab ← and → go to from at of n, round the ends; any other key stays
export function nextTab(at: number, n: number, key: string): number {
  if (n === 0) return at;
  if (key === 'ArrowRight') return (at + 1) % n;
  if (key === 'ArrowLeft') return (at - 1 + n) % n;
  return at;
}
