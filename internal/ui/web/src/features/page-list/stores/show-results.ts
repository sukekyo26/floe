import { columns as pageColumns, type ColumnsStore } from '@/stores/columns';

// The width from which the list can be folded beside the page
const wideNow = () => window.matchMedia('not (max-width: 70rem)').matches;

// showResults opens the list folded, as what the shelf and the search pick is the list's to show: on the wide page
// alone, where it can be folded
export function showResults(columns: ColumnsStore = pageColumns, wide: () => boolean = wideNow): void {
  if (!wide() || columns.listOpen()) return;
  columns.show('list', true);
}
