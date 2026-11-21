// edgeStep is the width a key moves an edge to: 16 px either way from the width shown, Home and End its bounds; null
// for any other key
export function edgeStep(key: string, now: number, min: number, max: number): number | null {
  switch (key) {
    case 'ArrowLeft':
      return now - 16;
    case 'ArrowRight':
      return now + 16;
    case 'Home':
      return min;
    case 'End':
      return max;
    default:
      return null;
  }
}
