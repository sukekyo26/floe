import { useLayoutEffect, useRef, useState } from 'react';

import { Button } from '@/components/button';
import { DoneIcon } from '@/components/icons';
import { useStore } from '@/hooks/use-store';
import {
  pressToastButton,
  toasts as pageToasts,
  type Toast,
  isToastLink,
  type ToastButton,
  type ToastLink,
  type Toasts as ToastStore,
} from '@/stores/toasts';

// ToastAction is a toast's button: held while its press goes when it holds, Escape its own
function ToastAction({ button }: { readonly button: ToastButton }) {
  const [going, setGoing] = useState(false);
  return (
    <span className="ml-2 flex">
      <Button
        onKeyDown={(e) => {
          if (e.key !== 'Escape' || !button.onEscape) return;
          e.stopPropagation();
          button.onEscape();
        }}
        testid={button.undo ? 'undo' : undefined}
        slug={button.slug}
        label={button.name}
        disabled={going}
        onClick={() => {
          pressToastButton(button, setGoing);
        }}
      >
        {button.label}
      </Button>
    </span>
  );
}

// ToastAnchor is a toast's link: the toast takes no pointer, the link does
function ToastAnchor({ link }: { readonly link: ToastLink }) {
  return (
    <a data-testid="toast-link" href={link.href} className="pointer-events-auto text-inherit">
      {link.label}
    </a>
  );
}

// ToastView is a toast: one with a button holds while pointed at or focused, and has its whole time again after
function ToastView({ toast, store }: { readonly toast: Toast; readonly store: ToastStore }) {
  // Not :hover, which stays on a touch screen after the finger has gone
  const over = useRef(false);
  const box = useRef<HTMLSpanElement>(null);
  const { button } = toast;
  // A change made from the keys gives its way back the focus as it is drawn, once: a toast drawn after takes nothing
  useLayoutEffect(() => {
    if (toast.focus) box.current?.querySelector('button')?.focus();
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- once, as it is first drawn
  }, []);
  return (
    <span
      ref={box}
      data-testid="toast"
      data-timed={toast.timed || undefined}
      data-page={toast.page || undefined}
      // Its id, for the focus to be given to its button from outside once a modal over it closes (Set the task's way off)
      data-toast={toast.id}
      onPointerEnter={
        button
          ? () => {
              over.current = true;
              store.hold(toast.id);
            }
          : undefined
      }
      onPointerLeave={
        button
          ? (e) => {
              over.current = false;
              if (!e.currentTarget.contains(document.activeElement)) store.again(toast.id);
            }
          : undefined
      }
      onFocus={
        button
          ? () => {
              store.focused.set(toast.id);
              store.hold(toast.id);
            }
          : undefined
      }
      onBlur={
        button
          ? () => {
              if (store.focused.get() === toast.id) store.focused.set(null);
              if (!over.current) store.again(toast.id);
            }
          : undefined
      }
      className={`flex items-center gap-2 rounded-corner border border-line bg-panel pl-3.75 text-small font-[550] text-fg shadow-md [&>svg]:size-4 [&>svg]:flex-none [&>svg]:text-good ${button && !isToastLink(button) ? 'pointer-events-auto py-1.5 pr-1.5' : 'py-2.5 pr-3.75'} ${toast.timed ? 'max-w-[calc(100vw-32px)] animate-toast-in' : 'animate-toast'}`}
    >
      <DoneIcon />
      {toast.text}
      {button === null ? null : isToastLink(button) ? <ToastAnchor link={button} /> : <ToastAction button={button} />}
    </span>
  );
}

// Toasts is where the toasts stand, bottom right, read out as they come; only one with a button takes the pointer
export function Toasts({ store = pageToasts }: { readonly store?: ToastStore }) {
  const shown = useStore(store.shown);
  return (
    // Its own box, always there: a modal open moves #toasts out of it and back into it (useMovedOverlay)
    <div data-home="toasts" className="contents">
      {/* An output is a status region (role status), read as it changes: it is never drawn anew, only what is in it */}
      <output
        id="toasts"
        aria-live="polite"
        className="pointer-events-none fixed right-4 bottom-5 z-40 grid justify-items-end gap-2"
      >
        {shown.map((x) => (
          <ToastView key={x.id} toast={x} store={store} />
        ))}
      </output>
    </div>
  );
}
