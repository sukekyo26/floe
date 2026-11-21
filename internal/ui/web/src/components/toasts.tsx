import { useRef } from 'react';

import { DoneIcon } from '@/components/icons';
import { useStore } from '@/hooks/use-store';
import { toasts as pageToasts, type Toast, type Toasts as ToastStore } from '@/stores/toasts';

// ToastView is a toast: one with a button holds while pointed at or focused, and has its whole time again after
function ToastView({ toast, store }: { readonly toast: Toast; readonly store: ToastStore }) {
  // Not :hover, which stays on a touch screen after the finger has gone
  const over = useRef(false);
  const holds = toast.button !== null;
  return (
    <span
      data-testid="toast"
      data-timed={toast.timed || undefined}
      data-page={toast.page || undefined}
      onPointerEnter={
        holds
          ? () => {
              over.current = true;
              store.hold(toast.id);
            }
          : undefined
      }
      onPointerLeave={
        holds
          ? (e) => {
              over.current = false;
              if (!e.currentTarget.contains(document.activeElement)) store.again(toast.id);
            }
          : undefined
      }
      onFocus={
        holds
          ? () => {
              store.hold(toast.id);
            }
          : undefined
      }
      onBlur={
        holds
          ? () => {
              if (!over.current) store.again(toast.id);
            }
          : undefined
      }
      className={`flex items-center gap-2 rounded-corner border border-line bg-panel pl-3.75 text-small font-[550] text-fg shadow-md [&>svg]:size-4 [&>svg]:flex-none [&>svg]:text-good ${holds ? 'pointer-events-auto py-1.5 pr-1.5' : 'py-2.5 pr-3.75'} ${toast.timed ? 'max-w-[calc(100vw-32px)] animate-toast-in' : 'animate-toast'}`}
    >
      <DoneIcon />
      {toast.text}
      {holds ? <span className="ml-2 flex">{toast.button}</span> : null}
    </span>
  );
}

// Toasts is where the toasts stand, bottom right, read out as they come; only one with a button takes the pointer
export function Toasts({ store = pageToasts }: { readonly store?: ToastStore }) {
  const shown = useStore(store.shown);
  return (
    // An output is a status region (role status), read as it changes: it is never drawn anew, only what is in it
    <output
      id="toasts"
      aria-live="polite"
      className="pointer-events-none fixed right-4 bottom-5 z-40 grid justify-items-end gap-2"
    >
      {shown.map((x) => (
        <ToastView key={x.id} toast={x} store={store} />
      ))}
    </output>
  );
}
