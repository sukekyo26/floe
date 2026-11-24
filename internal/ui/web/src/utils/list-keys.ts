// nextReached is the option ↓ and ↑ reach from active (-1 for none) of n, round the ends; with none to reach, or any
// other key, it stays (the search's rows, and the suggestions of a field)
export function nextReached(active: number, n: number, key: string): number {
  if (n === 0) return active;
  if (key === 'ArrowDown') return (active + 1) % n;
  if (key === 'ArrowUp') return (active <= 0 ? n : active) - 1;
  return active;
}
