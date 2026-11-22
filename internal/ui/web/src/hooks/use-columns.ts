import { useRem, useWindowWidth } from '@/hooks/use-window-width';
import { useStore } from '@/hooks/use-store';
import { columns as pageColumns, type ColumnsStore, type KeptColumns } from '@/stores/columns';
import type { Screen } from '@/types/screen';
import { columnWidths, listShown, type ColumnWidths } from '@/utils/column-widths';

export interface ColumnState {
  screen: Screen;
  kept: KeptColumns;
  // at is what the widths are within: whether the shelf takes room, the window's width and the browser's text
  at: { shelfShown: boolean; innerWidth: number; rem: number };
  widths: ColumnWidths;
  shelfOpen: boolean;
  listShown: boolean;
  picked: boolean;
}

// useColumns is the columns as they are laid out now: what is kept, its widths within the bounds of the window now
// (what is kept stays as it is in a window narrower for now), and what shows
export function useColumns(columns: ColumnsStore = pageColumns): ColumnState {
  const kept = useStore(columns.columns);
  const picked = useStore(columns.picked) !== null;
  const innerWidth = useWindowWidth();
  const rem = useRem();
  const screen = columns.screen();
  const at = { shelfShown: kept.open && screen !== 'taskPage', innerWidth, rem };
  return {
    screen,
    kept,
    at,
    widths: columnWidths(kept, at),
    shelfOpen: kept.open,
    listShown: listShown(screen, kept.listOpen, picked),
    picked,
  };
}
