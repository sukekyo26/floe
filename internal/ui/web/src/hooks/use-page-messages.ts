import { useEffect, useEffectEvent, type RefObject } from 'react';

import { newKey } from '@/lib/press-rest';
import type { ReadMark } from '@/types/page';
import { pageMessage, type FrameKind } from '@/utils/page-message';

export interface PageMessageCalls {
  // givenPath is the path this page gave the iframe now, null for none
  givenPath: () => string | null;
  // shown tells the iframe is on the screen now; left out, it always is
  shown?: () => boolean;
  // keys are the screen's table of keys (pageMessage passes those of PAGE_KEYS alone)
  keys: ReadonlySet<string>;
  onKey: (key: string) => void;
  onOpen?: (href: string) => void;
  onChanged?: () => void;
  onHash?: (hash: string) => void;
  onServed?: (mark: ReadMark) => void;
}

// usePageMessages takes the messages of the page in frame (pageMessage decides), and tells the page, each time it is
// loaded, that its frame takes its keys (pageKeysWanted: a page passes none to a frame that does not)
export function usePageMessages(
  frame: RefObject<HTMLIFrameElement | null>,
  kind: FrameKind,
  calls: PageMessageCalls,
): void {
  const onMessage = useEffectEvent((e: MessageEvent) => {
    const f = frame.current;
    const actions = pageMessage(e.data, {
      frame: kind,
      fromShown: f !== null && e.source === f.contentWindow && (calls.shown?.() ?? true),
      origin: e.origin,
      givenPath: calls.givenPath(),
      focused: f !== null && document.activeElement === f,
      keys: calls.keys,
      here: window.location.origin,
    });
    for (const action of actions) {
      if (action.kind === 'key') {
        // Not back to the iframe when a modal it opened closes: a page that keeps sending would open it again, for good
        f?.blur();
        newKey();
        calls.onKey(action.key);
      } else if (action.kind === 'open') {
        if (calls.onOpen) calls.onOpen(action.href);
        else window.location.assign(action.href);
      } else if (action.kind === 'changed') calls.onChanged?.();
      else if (action.kind === 'hash') calls.onHash?.(action.hash);
      else calls.onServed?.(action.mark);
    }
  });
  const onLoad = useEffectEvent(() => {
    // The page is sandboxed, of origin null: * is the one way to reach it
    frame.current?.contentWindow?.postMessage({ pageKeysWanted: true }, '*');
  });
  useEffect(() => {
    const f = frame.current;
    const message = (e: MessageEvent) => {
      onMessage(e);
    };
    const load = () => {
      onLoad();
    };
    window.addEventListener('message', message);
    f?.addEventListener('load', load);
    return () => {
      window.removeEventListener('message', message);
      f?.removeEventListener('load', load);
    };
  }, [frame]);
}
