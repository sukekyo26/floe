import { useEffect } from 'react';

import { ONE_COLUMN } from '@/lib/platform';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';

// useOneColumn lets go of the page open beside the list as the window narrows to one column, which has nothing beside
// it: what stayed loaded there would reload itself, and be told read unseen. Called once by a screen
export function useOneColumn(openPage: OpenPage = pageOpen): void {
  useEffect(() => {
    const narrow = window.matchMedia(ONE_COLUMN);
    const change = () => {
      if (narrow.matches && openPage.picked.get() !== null) openPage.picked.set(null);
    };
    narrow.addEventListener('change', change);
    return () => {
      narrow.removeEventListener('change', change);
    };
  }, [openPage]);
}
