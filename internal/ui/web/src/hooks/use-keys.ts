import { useEffect, useEffectEvent } from 'react';

import { composing, diffPickKey, keyName, plainKey } from '@/utils/keys';

// The keys of the screen, each run by its name (keyName): one table, which the app makes of its features' keys
export type KeyTable = Readonly<Record<string, () => void>>;

// useKeys runs the screen's keys from the document: Ctrl+K from a field too, where the browser's own goes; the rest as
// plainKey says, or from a choice of versions. A key the table has is the page's: the browser does nothing more with it
export function useKeys(table: KeyTable): void {
  const onKey = useEffectEvent((e: KeyboardEvent) => {
    if (composing(e)) return;
    const target = e.target instanceof Element ? e.target : null;
    const name = keyName(e);
    const ours =
      name === 'Ctrl+K' ||
      plainKey(
        { key: e.key, ctrlKey: e.ctrlKey, metaKey: e.metaKey, altKey: e.altKey, target },
        document.querySelector('dialog[open]') !== null,
      ) ||
      diffPickKey({ key: e.key, ctrlKey: e.ctrlKey, metaKey: e.metaKey, altKey: e.altKey, target });
    if (!ours || !Object.hasOwn(table, name)) return;
    e.preventDefault();
    table[name]?.();
  });
  useEffect(() => {
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('keydown', onKey);
    };
  }, []);
}
