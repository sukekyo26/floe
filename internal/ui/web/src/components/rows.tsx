import { Component, createRef, Fragment, type ReactNode } from 'react';

import { pages as pageStore, type Pages } from '@/stores/pages';
import { focusTarget, type FocusOn, type Focused } from '@/utils/row-focus';

// What Rows draws, in the order the caller arranged (arrange): a group's heading, or a row of an entry, which render
// makes, told whether it was drawn before
export type RowItem =
  | {
      readonly kind: 'group';
      readonly key: string;
      readonly heading: string;
      readonly empty?: string;
    }
  | {
      readonly kind: 'row';
      readonly key: string;
      readonly titled: boolean;
      readonly render: (kept: boolean) => ReactNode;
    };

export interface RowsProps {
  readonly items: readonly RowItem[];
  // selecting: rows are selected, which shows every row's box
  readonly selecting: boolean;
  readonly pages?: Pages;
}

// What may hold the focus in a row, by what it is
const FOCUS: Record<FocusOn, string> = {
  title: '[data-testid=title]',
  summary: 'summary',
  star: '[data-testid=star]',
  archive: '[data-testid=actions] > [data-testid=archive]',
};

interface Snapshot {
  was: string[];
  focused: Focused | null;
}

function Heading({ item }: { readonly item: Extract<RowItem, { kind: 'group' }> }) {
  return (
    <li
      data-key={item.key}
      data-testid="group"
      role="none"
      className="block ps-[calc(1rem+1px)] pe-0 pt-2.75 pb-1 text-small leading-[1.2] font-bold text-muted"
    >
      {/* oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- the tests find it by its role */}
      <div role="heading" aria-level={2}>
        {item.heading}
      </div>
      {item.empty === undefined ? null : (
        <p data-testid="category-empty" className="m-0 mt-1 pb-2 font-normal">
          {item.empty}
        </p>
      )}
    </li>
  );
}

// Rows draws #list, of every screen alike, and keeps what had the focus there as it is drawn anew: a read redraws the
// list as the keys go through it. A press on the rows holds the polls' answers until it is let go (pressed, released),
// as Chrome sends neither click nor dblclick for a row gone between press and release
export class Rows extends Component<RowsProps> {
  private readonly list = createRef<HTMLUListElement>();
  // The keys drawn, of the last commit: a row new to the list rises in, one drawn before is drawn in place
  private drawn: ReadonlySet<string> = new Set();
  private readonly released = () => {
    (this.props.pages ?? pageStore).released();
  };

  override componentDidMount() {
    this.drawn = new Set(this.props.items.map((x) => x.key));
    window.addEventListener('pointerup', this.released);
  }

  override componentWillUnmount() {
    window.removeEventListener('pointerup', this.released);
  }

  // Taken just before React changes the rows: which row and what of it had the focus
  override getSnapshotBeforeUpdate(prev: RowsProps): Snapshot {
    const was = prev.items.map((x) => x.key);
    const active = document.activeElement;
    const list = this.list.current;
    if (!list || !active || !list.contains(active)) return { was, focused: null };
    const row = active.closest('li[data-key]');
    const on = (Object.keys(FOCUS) as FocusOn[]).find((k) => active.matches(FOCUS[k]));
    const key = row?.getAttribute('data-key');
    return { was, focused: key && on ? { key, on } : null };
  }

  override componentDidUpdate(_prev: RowsProps, _state: unknown, snap: Snapshot) {
    const { items } = this.props;
    this.drawn = new Set(items.map((x) => x.key));
    if (!snap.focused) return;
    const to = focusTarget(
      snap.was,
      items.map((x) => ({ key: x.key, titled: x.kind === 'row' && x.titled })),
      snap.focused,
    );
    if (!to) return;
    const el = this.list.current?.querySelector(`li[data-key="${CSS.escape(to.key)}"] ${FOCUS[to.on]}`);
    // A row React moved has lost the focus, and one gone left it on the body
    if (el instanceof HTMLElement && el !== document.activeElement) el.focus({ preventScroll: true });
  }

  override render() {
    const { items, selecting } = this.props;
    const pages = this.props.pages ?? pageStore;
    return (
      <ul
        id="list"
        ref={this.list}
        data-selecting={selecting || undefined}
        onPointerDown={() => {
          pages.pressed();
        }}
        className="m-0 list-none px-1.5 pt-1.5 pb-4 empty:p-0"
      >
        {items.map((x) =>
          x.kind === 'group' ? (
            <Heading key={x.key} item={x} />
          ) : (
            <Fragment key={x.key}>{x.render(this.drawn.has(x.key))}</Fragment>
          ),
        )}
      </ul>
    );
  }
}
