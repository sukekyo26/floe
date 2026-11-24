import {
  useCallback,
  useLayoutEffect,
  useRef,
  useState,
  type FocusEvent,
  type KeyboardEvent,
  type MouseEvent,
  type ReactNode,
  type RefObject,
} from 'react';

import { givingBack } from '@/components/dialog';
import { composing } from '@/utils/keys';
import { suggestKey, type Offer } from '@/utils/name-offers';

export interface NameSuggestWays {
  // id is the field's: the box of names is `${id}-names`, each name `${id}-names-${i}`
  readonly id: string;
  // offers is what to show of what is written; typed tells it was just typed (nameOffers)
  readonly offers: (written: string, typed: boolean) => Offer[];
  // put puts a name in the field
  readonly put: (name: string) => void;
  // onFocus tells whether being reached opens it: a modal's first field, which it focuses as it opens, does not
  readonly onFocus?: boolean;
  // input is the caller's own ref to the field, which field.ref fills too: the field takes one ref
  readonly input?: RefObject<HTMLInputElement | null>;
}

export interface NameSuggest {
  // field is what the field takes: its combobox attributes and the handlers that open, move and close the names
  readonly field: {
    role: 'combobox';
    'aria-autocomplete': 'list';
    'aria-controls': string;
    'aria-expanded': boolean;
    'aria-activedescendant': string | undefined;
    onInput: (e: { currentTarget: HTMLInputElement }) => void;
    onKeyDown: (e: KeyboardEvent<HTMLInputElement>) => void;
    onFocus: (e: FocusEvent<HTMLInputElement>) => void;
    onClick: (e: MouseEvent<HTMLInputElement>) => void;
    // ref closes the names as the field is left
    ref: (input: HTMLInputElement | null) => (() => void) | undefined;
  };
  // box is the names, drawn out of the field's label (a press in the label is sent on to the field)
  readonly box: ReactNode;
}

// useNameSuggest suggests names for a field, as a WAI-ARIA combobox: ↓ opens it, ↑↓ reach a name, Enter or a press puts
// it in, Escape closes it (not the modal), as does leaving the field. The names are taken only as the field changes or
// is reached, so that a list read again neither closes them nor moves the name reached. They stand in the top layer (a
// popover), so that no modal or box cuts them off, under the field
export function useNameSuggest({ id, offers, put, onFocus = true, input: ownRef }: NameSuggestWays): NameSuggest {
  const [names, setNames] = useState<Offer[]>([]);
  const [shown, setShown] = useState(false);
  const [reached, setReached] = useState(-1);
  const typed = useRef(false);
  const box = useRef<HTMLDivElement>(null);
  const boxId = `${id}-names`;
  useLayoutEffect(() => {
    if (reached >= 0) document.getElementById(`${boxId}-${String(reached)}`)?.scrollIntoView({ block: 'nearest' });
  }, [boxId, reached]);

  // Under the field, and again once its modal has stopped moving: measured while it opens, the field is where it passes
  const place = async (input: HTMLInputElement) => {
    const at = () => {
      const r = input.getBoundingClientRect();
      const b = box.current;
      if (!b) return;
      b.style.left = `${String(r.left)}px`;
      b.style.top = `${String(r.bottom + 4)}px`;
      b.style.width = `${String(r.width)}px`;
    };
    at();
    // Settled, not all: an animation cancelled (the modal closed while it opens) rejects its finished
    await Promise.allSettled((input.closest('dialog')?.getAnimations() ?? []).map((a) => a.finished));
    at();
  };
  const close = useCallback(() => {
    if (box.current?.matches(':popover-open')) box.current.hidePopover();
    setShown(false);
    setReached(-1);
  }, []);
  // The field's own blur, not React's onBlur: a field disabled as its modal is held is left within React's commit, whose
  // events React does not send
  const leave = useCallback(
    (input: HTMLInputElement | null) => {
      if (ownRef) ownRef.current = input;
      if (!input) return undefined;
      input.addEventListener('blur', close);
      return () => {
        input.removeEventListener('blur', close);
        if (ownRef) ownRef.current = null;
      };
    },
    [close, ownRef],
  );
  const open = (input: HTMLInputElement) => {
    const now = offers(input.value, typed.current);
    typed.current = false;
    setNames(now);
    setReached(-1);
    if (now.length === 0) {
      close();
      return;
    }
    if (!box.current?.matches(':popover-open')) box.current?.showPopover();
    setShown(true);
    void place(input);
  };
  const choose = (name: string) => {
    put(name);
    close();
  };

  return {
    field: {
      role: 'combobox',
      'aria-autocomplete': 'list',
      'aria-controls': boxId,
      'aria-expanded': shown,
      'aria-activedescendant': reached >= 0 ? `${boxId}-${String(reached)}` : undefined,
      onInput: (e) => {
        typed.current = true;
        open(e.currentTarget);
      },
      onKeyDown: (e) => {
        if (composing(e.nativeEvent)) return;
        const what = suggestKey(e.key, shown, reached, names.length);
        if (what === null) return;
        // Tab goes on to the next field; the rest are the names' own
        if (e.key !== 'Tab') e.preventDefault();
        if (what.act === 'open') open(e.currentTarget);
        else if (what.act === 'reach') setReached(what.at);
        else if (what.act === 'put') choose(names[what.at]?.name ?? '');
        else {
          // Escape closes the names, not the modal
          if (e.key === 'Escape') e.stopPropagation();
          close();
        }
      },
      // Pressed or reached, the names show at once; pressed, again once the caret is where it was pressed
      onFocus: (e) => {
        if (onFocus && !givingBack()) open(e.currentTarget);
      },
      onClick: (e) => {
        open(e.currentTarget);
      },
      ref: leave,
    },
    box: (
      // Not ul and li, which the list's rows style
      <div
        ref={box}
        id={boxId}
        data-testid="suggest"
        popover="manual"
        // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- a field's names as a combobox's listbox: a datalist is not styled, nor does it say a name's notes
        role="listbox"
        className="fixed inset-auto m-0 hidden max-h-56 gap-px overflow-y-auto rounded-menu border border-line bg-panel p-menu-pad text-fg shadow-md open:grid"
      >
        {names.map((one, i) => (
          // oxlint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/interactive-supports-focus -- reached by the field's keys (aria-activedescendant), the focus staying on the field
          <div
            key={one.name}
            id={`${boxId}-${String(i)}`}
            // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- an option of the listbox above
            role="option"
            aria-selected={i === reached}
            title={[one.label, one.note].filter(Boolean).join(' ')}
            // Pressed, the field keeps its focus
            onMouseDown={(e) => {
              e.preventDefault();
            }}
            onClick={() => {
              choose(one.name);
            }}
            className={`cursor-pointer truncate rounded-[calc(var(--radius-menu)-var(--spacing-menu-pad))] px-[0.6rem] py-[0.4rem] text-small leading-[1.4] font-normal hover:bg-subtle aria-selected:bg-subtle aria-selected:text-fg aria-selected:shadow-[inset_2px_0_0_var(--color-ink)] ${one.note ? 'flex gap-[0.35rem]' : ''}`}
          >
            <span data-testid="names-word" className="min-w-0 truncate">
              {one.label}
            </span>
            {one.note ? (
              <span data-testid="names-note" className="flex-none text-muted">
                {one.note}
              </span>
            ) : null}
          </div>
        ))}
      </div>
    ),
  };
}
