import { Component, type ErrorInfo, type ReactNode } from 'react';

import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { pages as pageStore, type Pages } from '@/stores/pages';

interface GuardProps {
  // attempt counts the reads of the list ended: a screen that failed to draw tries again after the next
  readonly attempt: number;
  readonly pages: Pages;
  readonly children: ReactNode;
}

export interface GuardState {
  // failedAt is the read after which the draw failed (-1 until it is read from the props), null while it draws
  readonly failedAt: number | null;
}

// Guard catches what the screen throws as it draws, and says it with the load's mistakes, the read's first
// LIMIT: a draw that always throws is tried and said again after every read, every POLL_MS
// LIMIT: it catches only what throws as React draws; an event's or an effect's mistake is its caller's to say
// LIMIT: while it has caught one the screen shows nothing of what is in it
export class Guard extends Component<GuardProps, GuardState> {
  override state: GuardState = { failedAt: null };

  static getDerivedStateFromError(): GuardState {
    return { failedAt: -1 };
  }

  static getDerivedStateFromProps(props: GuardProps, state: GuardState): GuardState | null {
    if (state.failedAt === -1) return { failedAt: props.attempt };
    if (state.failedAt !== null && props.attempt !== state.failedAt) return { failedAt: null };
    return null;
  }

  override componentDidCatch(error: Error, _info: ErrorInfo) {
    this.props.pages.drawFailed(t('ui_screen_draw_failed', String(error)));
  }

  override componentDidMount() {
    this.drew();
  }

  override componentDidUpdate() {
    this.drew();
  }

  // Drawn, a mistake said of a draw before is taken back (drawFailed tells no one of the same said again)
  private drew() {
    if (this.state.failedAt === null) this.props.pages.drawFailed('');
  }

  override render() {
    return this.state.failedAt === null ? this.props.children : null;
  }
}

// DrawGuard holds a screen's draw, one for the screen: ErrorBar stands outside it
export function DrawGuard({ pages = pageStore, children }: { readonly pages?: Pages; readonly children: ReactNode }) {
  const attempt = useStore(pages.ended);
  return (
    <Guard attempt={attempt} pages={pages}>
      {children}
    </Guard>
  );
}
