import { useEffect, useRef } from 'react';

// useBackToRow gives the row come back to its focus once, after the first list taken is drawn: read tells a list taken,
// which a read that failed or was held back is not; back is focus-places' backToRow, given by the screen of rows that
// calls it, once
export function useBackToRow(read: boolean, back: () => void): void {
  const done = useRef(false);
  useEffect(() => {
    if (!read || done.current) return;
    done.current = true;
    back();
  }, [read, back]);
}
