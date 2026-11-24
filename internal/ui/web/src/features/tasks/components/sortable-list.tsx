import { useEffect, useLayoutEffect, useRef, useState } from 'react';

import { IconButton } from '@/components/icon-button';
import { GripIcon, RowDownIcon, RowDropIcon, RowUpIcon } from '@/components/icons';
import { droppedTo, dropsAfter, ends, focusStep, moved, nearDropped, type Step } from '@/features/tasks/utils/sortable';

export interface SortableListProps {
  readonly id: string;
  readonly rows: readonly string[];
  // kind names a row by its URL (data-url) or its category (data-category), and its x (url-drop, category-drop)
  readonly kind: 'url' | 'category';
  readonly labelledBy: string;
  readonly describedBy?: string;
  // note is the words after a row's name, small ('' for none: a category with pages)
  readonly note?: (name: string) => string;
  readonly label: Record<Step | 'drop', (name: string) => string>;
  // unpressable tells a row whose x does nothing but say why (a category with pages)
  readonly unpressable?: (name: string) => boolean;
  // onMove takes the rows reordered, name the one moved
  readonly onMove: (rows: string[], name: string) => void;
  // onDrop takes the row's x pressed: off, or, unpressable, said why
  readonly onDrop: (name: string) => void;
  // onNone gives the focus where it goes once the last row is off: the way to add one
  readonly onNone: () => void;
}

// What takes the focus once the rows are drawn again: a row's button, or the row near one taken off
type Want = { name: string; button: string } | { dropped: number } | null;

const ROW =
  'flex items-center gap-2 bg-panel py-[0.2rem] pr-[0.4rem] pl-[0.6rem] [&+li]:border-t [&+li]:border-line data-[drop=before]:shadow-[inset_0_2px_0_var(--color-ink)] data-[drop=after]:shadow-[inset_0_-2px_0_var(--color-ink)]';

// SortableList is a list of the task edit's: its rows reordered by their buttons up and down, and by a drag from the
// grip where there is a pointer that hovers, the row going before the row it is dropped on, or after it in its lower
// half. Each list holds its own drag, so that a row of one is not dropped in another. A row is draggable from the press
// on its grip on, written on the element at once: a drag starts in the press's task, before React would draw it
export function SortableList({
  id,
  rows,
  kind,
  labelledBy,
  describedBy,
  note = () => '',
  label,
  unpressable = () => false,
  onMove,
  onDrop,
  onNone,
}: SortableListProps) {
  const list = useRef<HTMLOListElement>(null);
  const dragged = useRef<HTMLLIElement | null>(null);
  const want = useRef<Want>(null);
  const [mark, setMark] = useState<{ name: string; after: boolean } | null>(null);
  const dropTestid = kind === 'url' ? 'url-drop' : 'category-drop';

  // A press let go without a drag, anywhere, leaves no row draggable from anywhere but its grip; the row dragged is let
  // go by dragend. Not on pointercancel, which a browser may send before dragstart, and the drag would not start
  // LIMIT: a press cancelled without a drag (pointercancel alone) leaves its row draggable till the next press is let go
  // or the modal is drawn again
  useEffect(() => {
    const up = () => {
      for (const li of list.current?.children ?? [])
        if (li !== dragged.current && li instanceof HTMLElement) li.draggable = false;
    };
    document.addEventListener('pointerup', up);
    return () => {
      document.removeEventListener('pointerup', up);
    };
  }, []);

  // The focus goes back to the button pressed, as moving the row drew it anew, or to a button that can be pressed of
  // the row near one taken off, else to the way to add one
  useLayoutEffect(() => {
    const w = want.current;
    want.current = null;
    if (w === null) return;
    const items = [...(list.current?.children ?? [])];
    if ('button' in w) {
      const li = items.find((x) => x.getAttribute(`data-${kind}`) === w.name);
      li?.querySelector<HTMLElement>(`[data-testid=${w.button}]`)?.focus();
      return;
    }
    const near = items[nearDropped(w.dropped, items.length)];
    const order = kind === 'url' ? ['url-drop'] : ['category-drop', 'category-up', 'category-down'];
    const to = order
      .map((testid) => near?.querySelector<HTMLButtonElement>(`[data-testid=${testid}]`))
      .find((b) => b && !b.disabled && b.getAttribute('aria-disabled') !== 'true');
    if (to) to.focus();
    else onNone();
  });

  const step = (name: string, at: number, pressed: Step) => {
    const next = moved(rows, name, pressed === 'up' ? at - 1 : at + 2);
    want.current = { name, button: `category-${focusStep(pressed, next.indexOf(name), next.length)}` };
    onMove(next, name);
  };

  return (
    <ol
      ref={list}
      id={id}
      aria-labelledby={labelledBy}
      aria-describedby={describedBy}
      hidden={rows.length === 0}
      className="m-0 grid list-none p-0"
    >
      {rows.map((name, at) => {
        const end = ends(at, rows.length);
        return (
          // oxlint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- a row dropped on and dragged by its grip (HTML drag and drop, ui.md); its buttons move it by the keys
          <li
            key={name}
            {...{ [`data-${kind}`]: name }}
            data-drop={mark?.name === name ? (mark.after ? 'after' : 'before') : undefined}
            className={`${ROW} ${kind === 'url' ? 'font-normal' : 'font-semibold'}`}
            // A drag of words selected starts at their node; a row's own drag only from its grip
            onDragStart={(e) => {
              const li = e.currentTarget;
              if (!li.draggable) return;
              dragged.current = li;
              e.dataTransfer.effectAllowed = 'move';
              e.dataTransfer.setData('text/plain', name);
            }}
            onDragOver={(e) => {
              if (!dragged.current) return;
              e.preventDefault();
              if (e.currentTarget === dragged.current) {
                setMark(null);
                return;
              }
              const box = e.currentTarget.getBoundingClientRect();
              const after = dropsAfter(box.top, box.height, e.clientY);
              // The same mark kept: a drag over the same half draws nothing anew
              setMark((was) => (was?.name === name && was.after === after ? was : { name, after }));
            }}
            onDrop={(e) => {
              const from = dragged.current?.getAttribute(`data-${kind}`) ?? null;
              if (from === null) return;
              e.preventDefault();
              if (from === name) return;
              const box = e.currentTarget.getBoundingClientRect();
              const next = droppedTo(rows, from, dropsAfter(box.top, box.height, e.clientY) ? at + 1 : at);
              if (next) onMove(next, from);
            }}
            onDragEnd={() => {
              if (dragged.current) dragged.current.draggable = false;
              dragged.current = null;
              setMark(null);
            }}
          >
            <span
              data-testid="category-grip"
              className="inline-flex cursor-grab touch-none text-faint [&_svg]:size-3.5 [@media(hover:none)]:hidden"
              onPointerDown={(e) => {
                const li = e.currentTarget.closest('li');
                if (li) li.draggable = true;
              }}
            >
              <GripIcon />
            </span>
            <span
              data-testid="category-name"
              className={`min-w-0 flex-1 wrap-anywhere ${kind === 'url' ? 'font-mono' : ''}`}
            >
              {name}
              {note(name) ? (
                <>
                  {' '}
                  <small className="font-normal text-muted">{note(name)}</small>
                </>
              ) : null}
            </span>
            <span className="flex gap-[0.15rem]">
              <IconButton
                testid="category-up"
                label={label.up(name)}
                disabled={end.up}
                onClick={() => {
                  step(name, at, 'up');
                }}
              >
                <RowUpIcon />
              </IconButton>
              <IconButton
                testid="category-down"
                label={label.down(name)}
                disabled={end.down}
                onClick={() => {
                  step(name, at, 'down');
                }}
              >
                <RowDownIcon />
              </IconButton>
              <IconButton
                testid={dropTestid}
                label={label.drop(name)}
                unpressable={unpressable(name)}
                onClick={() => {
                  if (!unpressable(name)) want.current = { dropped: at };
                  onDrop(name);
                }}
              >
                <RowDropIcon />
              </IconButton>
            </span>
          </li>
        );
      })}
    </ol>
  );
}
