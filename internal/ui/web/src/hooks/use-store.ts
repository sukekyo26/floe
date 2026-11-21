import { useSyncExternalStore } from 'react';

import type { Store } from '@/lib/store';

// useStore is the value of a store, drawn anew as it changes
export const useStore = <T>(store: Store<T>): T => useSyncExternalStore(store.subscribe, store.get, store.get);
