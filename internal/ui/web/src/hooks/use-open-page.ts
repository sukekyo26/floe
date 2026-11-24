import { useStore } from '@/hooks/use-store';
import { openPage, type OpenAt } from '@/stores/open-page';

// useOpenPage is the row whose page is open beside the list, or null
export const useOpenPage = (): OpenAt | null => useStore(openPage.picked);
