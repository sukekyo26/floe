import { getBoot, type Boot } from '@/config/boot';
import { versions as frameVersions, type Versions } from '@/features/versions/stores/versions';
import { openPage as page, type OpenPage } from '@/stores/open-page';
import { order as pageOrder, type Order } from '@/stores/order';

interface Deps {
  boot?: () => Boot;
  versions?: Pick<Versions, 'arrive'>;
  openPage?: Pick<OpenPage, 'diff' | 'frameAt'>;
  order?: Pick<Order, 'keepAt'>;
  href?: () => string;
  replace?: (url: string) => void;
}

// startFrame is the frame's start, once before anything is drawn (an effect would run after the children's, twice in
// StrictMode): what a move of versions carried is taken, the rows that opened the frame keep where it is, and a ?v= the
// server read as no other version (the latest, or one gone) leaves the URL
export function startFrame({
  boot = getBoot,
  versions = frameVersions,
  openPage = page,
  order = pageOrder,
  href = () => window.location.href,
  replace = (url) => {
    window.history.replaceState(window.history.state, '', url);
  },
}: Deps = {}): void {
  versions.arrive();
  const at = openPage.frameAt();
  if (at) order.keepAt(at);
  const { version } = boot();
  if (openPage.diff() || !version || Number(version)) return;
  const url = new URL(href());
  url.searchParams.delete('v');
  replace(url.href);
}
