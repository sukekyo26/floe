import { createContext, use, useEffectEvent, useLayoutEffect, useRef, type ReactNode } from 'react';

import { Button } from '@/components/button';
import type { Hold } from '@/hooks/use-hold';
import { t } from '@/lib/t';
import { backdropDown } from '@/utils/dialog';
import { heldKey } from '@/utils/keys';

interface DialogScope {
  // close is the one way a modal closes (ui.md: closeDialog), for its close buttons
  close: () => void;
  // givingBack tells that the focus going to a field is the hold giving it back, not a field reached: its suggestions
  // stay shut, over the mistake said under it
  givingBack: () => boolean;
}

const Scope = createContext<DialogScope>({ close: () => undefined, givingBack: () => false });

// useDialog is the modal a part is in
export const useDialog = (): DialogScope => use(Scope);

export interface DialogProps {
  readonly id: string;
  readonly open: boolean;
  // onClosed tells the caller a person closed it (a close button, Escape, the backdrop): open is to be false
  readonly onClosed: () => void;
  // afterClose is where the focus goes as it closes, in the same task as close(): Chrome sends the close event an
  // animation frame later, and keys meanwhile would reach the closed modal's field, or the one under it
  readonly afterClose?: () => HTMLElement | null | undefined;
  // hold holds it while it writes (useHold)
  readonly hold?: Pick<Hold, 'held' | 'focus'>;
  readonly labelledBy: string;
  readonly describedBy?: string;
  readonly children: ReactNode;
}

const NOT_HELD = { held: false, focus: { current: null } } as const;

// Dialog is a modal: a dialog shown by showModal(), its head, body and foot in one fieldset, which a hold disables
// whole (its close buttons too), so that nothing closes it, sends again or starts another write; what follows the write
// is of the modal it was sent from, open as it was. Held, it takes no close request (closedby), no press (but on a link:
// LIMIT, as in ui.html) and no key but those heldKey lets go, and the focus is on the modal itself, given back once
// let go
export function Dialog({
  id,
  open,
  onClosed,
  afterClose,
  hold = NOT_HELD,
  labelledBy,
  describedBy,
  children,
}: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const was = useRef({ open: false, held: false });
  const giving = useRef(false);
  const downOutside = useRef(false);
  const { held } = hold;

  const shutNow = (d: HTMLDialogElement) => {
    d.close();
    afterClose?.()?.focus();
    if (d.contains(document.activeElement) && document.activeElement instanceof HTMLElement)
      document.activeElement.blur();
  };
  const shut = useEffectEvent(shutNow);
  const close = () => {
    const d = ref.current;
    if (!d?.open || d.getAttribute('closedby') === 'none') return;
    shutNow(d);
    onClosed();
  };
  const giveBack = useEffectEvent((d: HTMLDialogElement) => {
    const focus = hold.focus.current;
    const active = document.activeElement;
    if ((!d.contains(active) || active === d) && focus instanceof HTMLElement && d.contains(focus)) {
      giving.current = true;
      focus.focus();
      giving.current = false;
    }
  });

  // Let go first, then closed: the caller closes the modal once the hold has let go
  useLayoutEffect(() => {
    const d = ref.current;
    if (!d) return;
    const before = was.current;
    was.current = { open, held };
    if (held && !before.held) d.focus({ preventScroll: true });
    if (!held && before.held && open) giveBack(d);
    if (open && !d.open) d.showModal();
    if (!open && d.open) shut(d);
  }, [open, held]);

  return (
    <Scope value={{ close, givingBack: () => giving.current }}>
      {/* The backdrop is the dialog itself, pressed as Escape is keyed: a close request (cancel) is its keys' way */}
      {/* oxlint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-noninteractive-element-interactions -- a press on the backdrop closes it; keys close it by cancel */}
      <dialog
        ref={ref}
        id={id}
        aria-labelledby={labelledBy}
        aria-describedby={describedBy}
        closedby={held ? 'none' : 'closerequest'}
        tabIndex={held ? -1 : undefined}
        // Escape and the other close requests: the browser fires cancel within the key's task, before the next key,
        // and not while closedby none holds it
        onCancel={close}
        onPointerDown={(e) => {
          downOutside.current = backdropDown(e.target === e.currentTarget, held);
        }}
        onClick={(e) => {
          if (e.target === e.currentTarget && downOutside.current) close();
          downOutside.current = false;
        }}
        onKeyDownCapture={(e) => {
          if (!held || heldKey(e.nativeEvent)) return;
          e.preventDefault();
          e.stopPropagation();
        }}
        className="m-auto w-[min(32rem,calc(100%-32px))] rounded-corner border border-line bg-panel p-0 text-fg shadow-lg backdrop:bg-scrim backdrop:backdrop-blur-[3px] open:animate-pop open:backdrop:animate-fade [&[closedby=none]]:focus-visible:outline-none [&[closedby=none]>*]:pointer-events-none"
      >
        <fieldset disabled={held} className="m-0 grid min-w-0 gap-4 border-0 px-6 pt-5.5 pb-5">
          {children}
        </fieldset>
      </dialog>
    </Scope>
  );
}

export interface DialogHeadProps {
  readonly icon: ReactNode;
  readonly titleId: string;
  readonly title: ReactNode;
  readonly lead?: ReactNode;
}

// DialogHead is a modal's head: its mark in a circle, its title and a line under it
export function DialogHead({ icon, titleId, title, lead }: DialogHeadProps) {
  return (
    <div data-testid="modal-head" className="flex items-center gap-3.25">
      <span className="grid size-9.5 flex-none place-items-center rounded-full bg-sel p-2.25 text-fg [&_svg]:size-5">
        {icon}
      </span>
      <div>
        <h2 id={titleId} className="m-0 text-[1.05rem] leading-[1.3] font-bold tracking-[-0.01em]">
          {title}
        </h2>
        {lead === undefined ? null : (
          <p data-testid="lead" className="mt-0.5 mb-0 text-small text-muted">
            {lead}
          </p>
        )}
      </div>
    </div>
  );
}

// DialogActions is a modal's foot: its buttons, at the end of a line over it
export function DialogActions({ children }: { readonly children: ReactNode }) {
  return (
    <div data-testid="dialog-actions" className="mt-1 flex flex-wrap justify-end gap-2 border-t border-line pt-4">
      {children}
    </div>
  );
}

// DialogClose is a modal's close button
export function DialogClose() {
  const { close } = useDialog();
  return (
    <Button size="foot" closes onClick={close}>
      {t('ui_close')}
    </Button>
  );
}
