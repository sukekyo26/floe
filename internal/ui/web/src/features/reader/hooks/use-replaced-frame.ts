import { useEffectEvent, useLayoutEffect, useRef, type RefObject } from 'react';

// useReplacedFrame loads href in the iframe beside the list, in place of its history entry: moved through by j and k,
// Back is the tab's, and does not leave the rows on one page and the iframe on another. The same href again loads
// nothing; opened is told each new one
export function useReplacedFrame(frame: RefObject<HTMLIFrameElement | null>, href: string, opened: () => void): void {
  const loaded = useRef('about:blank');
  const onOpened = useEffectEvent(opened);
  useLayoutEffect(() => {
    const f = frame.current;
    if (!f || href === loaded.current) return;
    loaded.current = href;
    f.setAttribute('data-loaded', href);
    onOpened();
    f.contentWindow?.location.replace(href);
  }, [frame, href]);
}
