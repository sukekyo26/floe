import { createContext, use, useEffectEvent, useLayoutEffect, useRef, type ReactNode } from 'react';

import { Button, ButtonSizeOf } from '@/components/button';
import type { Hold } from '@/hooks/use-hold';
import { dropRest } from '@/lib/press-rest';
import { t } from '@/lib/t';
import type { FocusBack } from '@/types/focus';
import type { MessageKey } from '@/types/messages.gen';
import { backdropCloses, heldKey, stopsKey, type PressDown } from '@/utils/dialog';

interface DialogScope {
  // close is the one way a modal closes, for its close buttons
  close: () => void;
}

const Scope = createContext<DialogScope>({ close: () => undefined });

// One value, not the modal's Scope: a field's suggestions are asked for in the body of the part that draws the modal,
// outside its Scope. focus() runs the focus events of the one element it focuses before it returns, so the value is
// true only for that element, whichever modal is under or over it
let giving = false;

// givingBack tells that the focus going to a field is a hold giving it back, not a field reached: its suggestions stay
// shut, over the mistake said under it
export const givingBack = (): boolean => giving;

// whileGiving gives the focus back to field, as givingBack tells
export function whileGiving(field: { focus: () => void }): void {
  giving = true;
  try {
    field.focus();
  } finally {
    giving = false;
  }
}

// useDialog is the modal a part is in
export const useDialog = (): DialogScope => use(Scope);

export interface DialogProps {
  readonly id: string;
  readonly open: boolean;
  // onClosed tells the caller a person closed it (a close button, Escape, the backdrop): open is to be false
  readonly onClosed: () => void;
  // afterClose is where the focus goes as it closes, in the same task as close(): Chrome sends the close event an
  // animation frame later, and keys meanwhile would reach the closed modal's field, or the one under it
  readonly afterClose?: FocusBack | undefined;
  // hold holds it while it writes (useHold)
  readonly hold?: Pick<Hold, 'held' | 'focus' | 'holding'>;
  readonly labelledBy: string;
  readonly describedBy?: string;
  // width: base 32rem, mid 40rem (the settings' list), wide 64rem (the settings' themes)
  readonly width?: 'base' | 'mid' | 'wide';
  // body: grid, its parts one under another; help, high in the window, a column whose box of many lines scrolls alone;
  // fill, in the middle up to the window's height less 32px, a column whose one box shrinks (the settings' cards)
  readonly body?: 'grid' | 'help' | 'fill';
  // panel is the part shown (data-panel), as the settings tell it
  readonly panel?: string | undefined;
  // bodyId names the box that holds its head, body and foot (#filter-box)
  readonly bodyId?: string;
  // raised goes up to the top in a narrow window (the search: its rows under the keyboard of a phone)
  readonly raised?: boolean;
  readonly children: ReactNode;
}

const WIDTH = {
  base: 'w-[min(32rem,calc(100%-32px))]',
  mid: 'w-[min(40rem,calc(100%-32px))]',
  wide: 'w-[min(64rem,calc(100%-32px))]',
} as const;
// Where it stands and how high it goes: --tall is the most the column bodies take, read again by the fieldset less
// the dialog's border
const PLACE = {
  grid: 'm-auto',
  help: 'mx-auto mt-[6dvh] mb-auto max-h-(--tall) [--tall:calc(94dvh-1rem)]',
  fill: 'm-auto max-h-(--tall) [--tall:calc(100dvh-32px)]',
} as const;

// A modal that never writes holds nothing: as before holding was told
const NOT_HELD = { held: false, focus: { current: null }, holding: () => false } as const;

// Dialog is a modal: a dialog shown by showModal(), its head, body and foot in one fieldset, which a hold disables
// whole (its close buttons too), so that nothing closes it, sends again or starts another write; what follows the write
// is of the modal it was sent from, open as it was. Held, it takes no close request (closedby), no press (but on a
// link: LIMIT) and no key but those heldKey lets go, and the focus is on the modal itself, given back once let go
// LIMIT: opened and held in one render, d.focus() runs before showModal() and does nothing. No caller opens a modal
// held; give the focus after showModal() if one comes to
export function Dialog({
  id,
  open,
  onClosed,
  afterClose,
  hold = NOT_HELD,
  labelledBy,
  describedBy,
  width = 'base',
  body = 'grid',
  panel,
  raised = false,
  bodyId,
  children,
}: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const was = useRef({ open: false, held: false });
  const down = useRef<PressDown | null>(null);
  const { held } = hold;

  const shutNow = (d: HTMLDialogElement) => {
    d.close();
    dropRest();
    afterClose?.();
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
    if ((!d.contains(active) || active === d) && focus instanceof HTMLElement && d.contains(focus)) whileGiving(focus);
  });

  // Let go first, then closed: the caller closes the modal once the hold has let go
  useLayoutEffect(() => {
    const d = ref.current;
    if (!d) return;
    const before = was.current;
    was.current = { open, held };
    if (held && !before.held) d.focus({ preventScroll: true });
    if (!held && before.held && open) giveBack(d);
    if (open && !d.open) {
      d.showModal();
      dropRest();
      // React's autoFocus focuses on mounting, and a modal's parts are mounted closed: the one it names takes it here
      d.querySelector<HTMLElement>('[data-autofocus]')?.focus();
    }
    if (!open && d.open) shut(d);
  }, [open, held]);

  return (
    <Scope value={{ close }}>
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
        // Closed from outside (close() called on the element, as a test may): the caller is told, as by a close button.
        // The event comes a frame after the close: one of a close by this modal finds open false, or, opened again
        // meanwhile (Ctrl+K twice), the element open, and is dropped
        // LIMIT: closed and opened again in one task by script (close, open, close, open, at once), it stays closed: open
        // ends as it began, and React draws nothing anew. A person's keys and presses are tasks of their own
        onClose={(e) => {
          if (open && !e.currentTarget.open) onClosed();
        }}
        onPointerDown={(e) => {
          down.current = { onDialog: e.target === e.currentTarget, held };
        }}
        onClick={(e) => {
          if (backdropCloses(down.current, e.target === e.currentTarget)) close();
          down.current = null;
        }}
        onKeyDownCapture={(e) => {
          if (!stopsKey(held, hold.holding(), heldKey(e.nativeEvent))) return;
          e.preventDefault();
          e.stopPropagation();
        }}
        data-panel={panel}
        className={`${PLACE[body]} ${raised ? 'narrow:mt-4 narrow:mb-auto' : ''} ${WIDTH[width]} rounded-corner border border-line bg-panel p-0 text-fg shadow-lg backdrop:bg-scrim backdrop:backdrop-blur-[3px] open:animate-pop open:backdrop:animate-fade [&[closedby=none]]:focus-visible:outline-none [&[closedby=none]>*]:pointer-events-none`}
      >
        <fieldset
          id={bodyId}
          disabled={held}
          className={`m-0 min-w-0 gap-4 border-0 px-6 pt-5.5 pb-5 ${body === 'grid' ? 'grid' : 'flex max-h-[calc(var(--tall)-2px)] flex-col'}`}
        >
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
  // leadId names the line under the title, which says nothing and is hidden while it is ''
  readonly leadId?: string;
}

// DialogHead is a modal's head: its mark in a circle, its title and a line under it
export function DialogHead({ icon, titleId, title, lead, leadId }: DialogHeadProps) {
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
          <p id={leadId} data-testid="lead" hidden={lead === ''} className="mt-0.5 mb-0 text-small text-muted">
            {lead}
          </p>
        )}
      </div>
    </div>
  );
}

// DialogActions is a modal's foot: its buttons, at the end of a line over it, of the foot's size unless they name one.
// A menu in it opens over it (the search's foot: relative); testid is the foot's name, as the tests read it
export function DialogActions({
  children,
  testid = 'dialog-actions',
}: {
  readonly children: ReactNode;
  readonly testid?: string;
}) {
  return (
    <div
      data-testid={testid}
      className="relative mt-1 flex flex-wrap items-center justify-end gap-2 border-t border-line pt-4"
    >
      <ButtonSizeOf value="foot">{children}</ButtonSizeOf>
    </div>
  );
}

// DialogClose is a modal's close button: Close, or Cancel in a modal that sends (TestDialogCloseWords)
// focusOnOpen gives it the focus as the modal opens (the help's)
export function DialogClose({
  words = 'ui_close',
  focusOnOpen = false,
  id,
}: {
  readonly words?: MessageKey;
  readonly focusOnOpen?: boolean;
  readonly id?: string;
}) {
  const { close } = useDialog();
  return (
    <Button closes focusOnOpen={focusOnOpen} onClick={close} {...(id ? { id } : {})}>
      {t(words)}
    </Button>
  );
}
