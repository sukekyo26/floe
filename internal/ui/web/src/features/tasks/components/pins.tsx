import { Component, createRef } from 'react';

import { CloseIcon, HelpIcon, PinIcon } from '@/components/icons';
import { IconButton } from '@/components/icon-button';
import { pinFocus, pinsSaid, type PinFocused, type PinPart } from '@/features/tasks/utils/pin-focus';
import { t } from '@/lib/t';
import { order as pageOrder, type Order } from '@/stores/order';
import { pageActions as pageActs, type PageActions } from '@/stores/page-actions';
import { pages as pageStore, type Pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import { pageHref } from '@/utils/page-urls';

export interface PinsProps {
  // pinned are the project's pages pinned (pinnedOf); hidden, for no project yet (the tasks unreadable, none kept)
  readonly pinned: readonly Page[];
  readonly hidden: boolean;
  readonly pages?: Pages;
  readonly order?: Order;
  readonly actions?: Pick<PageActions, 'togglePin'>;
}

interface Snapshot {
  was: string[];
  focused: PinFocused | null;
}

// Pins is the row of the pages pinned of the task list's project, and how to pin one when none is. The chips are drawn
// anew whole as soon as anything they say changes (their key holds pinsSaid), so that a screen reader says again the
// one with the focus; the focus stays on what had it, or of a chip gone, goes to the title of the one in its place
// (pinFocus). A press on them holds the polls' answers till it is let go, as the rows' does
// LIMIT: a folder pinned leads to the page list, where it opens (its URL), and not back here
export class Pins extends Component<PinsProps> {
  private readonly list = createRef<HTMLSpanElement>();
  private readonly released = () => {
    (this.props.pages ?? pageStore).released();
  };

  override componentDidMount() {
    window.addEventListener('pointerup', this.released);
  }

  override componentWillUnmount() {
    window.removeEventListener('pointerup', this.released);
  }

  override getSnapshotBeforeUpdate(prev: PinsProps): Snapshot {
    const was = prev.pinned.map((a) => a.slug);
    const active = document.activeElement;
    const chip = active?.closest('[data-slug]');
    const slug = chip?.getAttribute('data-slug');
    if (!this.list.current?.contains(active) || !slug) return { was, focused: null };
    const on: PinPart = active?.getAttribute('data-testid') === 'pin-off' ? 'pin-off' : 'pin';
    return { was, focused: { slug, on } };
  }

  override componentDidUpdate(_prev: PinsProps, _state: unknown, snap: Snapshot) {
    if (!snap.focused) return;
    const to = pinFocus(
      snap.was,
      this.props.pinned.map((a) => a.slug),
      snap.focused,
    );
    const el =
      to === 'help'
        ? document.getElementById('pins-help')
        : this.list.current?.querySelector(`[data-slug="${CSS.escape(to.slug)}"] [data-testid=${to.on}]`);
    // A chip drawn anew has lost the focus, and one gone left it on the body
    if (el instanceof HTMLElement && el !== document.activeElement) el.focus({ preventScroll: true });
  }

  override render() {
    const { pinned, hidden } = this.props;
    const pages = this.props.pages ?? pageStore;
    const order = this.props.order ?? pageOrder;
    const actions = this.props.actions ?? pageActs;
    const said = pinsSaid(pinned);
    // j and k go through the pins, and the way back is to this list
    const keep = () => {
      order.keepRows(
        pinned.filter((x) => !x.folder).map((x) => x.slug),
        pinned.map((x) => x.slug),
      );
    };
    return (
      <>
        <div
          id="pins"
          hidden={hidden}
          className="flex flex-none flex-wrap items-center gap-x-2 gap-y-[0.4rem] border-b border-line bg-bg px-4 py-[0.6rem] text-small narrow:[scrollbar-width:none] narrow:flex-nowrap narrow:overflow-x-auto"
        >
          <h2 className="m-0 me-[0.4rem] text-label leading-[1.2] font-semibold whitespace-nowrap text-muted">
            {t('ui_pins')}
          </h2>
          <span className="-ms-[0.4rem] flex">
            <IconButton id="pins-help" size="help" label={t('ui_pins_help')} popoverTarget="pins-tip">
              <HelpIcon />
            </IconButton>
          </span>
          <span
            id="pin-list"
            ref={this.list}
            className="contents"
            onPointerDown={() => {
              pages.pressed();
            }}
          >
            {pinned.length === 0 ? (
              <span data-testid="pins-none" className="text-muted">
                {t('ui_pins_none')}
              </span>
            ) : (
              pinned.map((a) => (
                <span
                  key={`${said}\n${a.slug}`}
                  data-slug={a.slug}
                  className="inline-flex max-w-72 items-center rounded-full border border-line bg-panel hover:border-faint narrow:max-w-52 narrow:flex-none"
                >
                  <a
                    data-testid="pin"
                    data-unread={a.news ? 'true' : undefined}
                    href={pageHref({ slug: a.slug })}
                    title={a.title}
                    onClick={keep}
                    className="inline-flex min-w-0 items-center gap-[0.35rem] rounded-full py-[0.18rem] ps-[0.65rem] pe-[0.1rem] text-fg no-underline data-unread:font-bold [&>svg]:size-3.25 [&>svg]:flex-none [&>svg]:text-muted"
                  >
                    <PinIcon />
                    <span data-testid="pin-title" className="min-w-0 truncate">
                      {a.title}
                    </span>
                    {a.news ? <span data-testid="dot" className="size-2 flex-none rounded-full bg-new" /> : null}
                  </a>
                  <IconButton
                    testid="pin-off"
                    size="chip"
                    label={t('ui_unpin_named', a.title ?? a.slug)}
                    // The second press of a double one lands on the next pin, moved under it, and no pin comes back but
                    // by its page's menu
                    onClick={(e) => {
                      if (e.detail <= 1) void actions.togglePin(a);
                    }}
                  >
                    <CloseIcon />
                  </IconButton>
                </span>
              ))
            )}
          </span>
        </div>
        {/* In the top layer: a narrow screen's row of pins scrolls, and would cut it off. Placed under its button as it
            opens; the sheet keeps it within the window */}
        <div
          id="pins-tip"
          popover="auto"
          onBeforeToggle={(e) => {
            if (e.newState !== 'open') return;
            const at = document.getElementById('pins-help')?.getBoundingClientRect();
            if (!at) return;
            const tip = e.currentTarget;
            tip.style.top = `${String(at.bottom + 4)}px`;
            tip.style.setProperty('--from', `${String(at.left)}px`);
          }}
          className="fixed inset-auto [left:max(16px,min(var(--from),100vw-16px-var(--width)))] m-0 w-(--width) rounded-menu border border-line bg-panel px-3 py-[0.6rem] text-small text-fg shadow-md [--width:min(20rem,100vw-2rem)]"
        >
          <strong>{t('ui_pins')}</strong>
          <p className="mt-1 mb-0 text-muted">{t('ui_pins_help_text')}</p>
        </div>
      </>
    );
  }
}
