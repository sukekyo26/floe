import type { ReactNode } from 'react';

import { useDialog } from '@/components/dialog';
import { PlusIcon } from '@/components/icons';
import { SHELF_MARK, SHELF_ROW } from '@/components/shelf-chip';

export interface ShelfAddProps {
  // hidden: none of its buttons shows (the caller knows which do)
  readonly hidden: boolean;
  readonly children: ReactNode;
}

// ShelfAdd holds the shelf's add buttons, under what they add, as one more row of the part above it when one shows
export function ShelfAdd({ hidden, children }: ShelfAddProps) {
  return (
    <div
      id="shelf-add"
      hidden={hidden}
      className="grid grid-cols-[minmax(0,1fr)] gap-1 [div:not([hidden])~&]:mt-[calc(4px-1.1rem)]"
    >
      {children}
    </div>
  );
}

export interface ShelfAddButtonProps {
  // id is the button's (add-link, add-task); short its words, for the shelf's narrowest; label the whole of what it
  // does, which holds those words
  readonly id: string;
  readonly short: string;
  readonly label: string;
  readonly hidden: boolean;
  readonly onAdd: () => void;
}

// ShelfAddButton opens what adds: pressed in the filters' modal, that closes first, its focus going back to the button
// that opened it, for the modal opened next to give it back there rather than to one hidden with the shelf. Out of a
// modal, the close does nothing
export function ShelfAddButton({ id, short, label, hidden, onAdd }: ShelfAddButtonProps) {
  const { close } = useDialog();
  return (
    <button
      type="button"
      id={id}
      aria-label={label}
      title={label}
      hidden={hidden}
      onClick={() => {
        close();
        onAdd();
      }}
      className={`flex ${SHELF_ROW} ${SHELF_MARK} text-label text-muted`}
    >
      <PlusIcon />
      {short}
    </button>
  );
}
