import { getBoot, type Boot } from '@/config/boot';
import { versions as frameVersions, type Versions } from '@/features/versions/stores/versions';
import { openPage as page, type OpenPage } from '@/stores/open-page';

interface Deps {
  boot?: () => Boot;
  versions?: Pick<Versions, 'arrive'>;
  openPage?: Pick<OpenPage, 'diff'>;
  href?: () => string;
  replace?: (url: string) => void;
}

// startFrame is the frame's start, once before anything is drawn (an effect would run after the children's, twice in
// StrictMode): what a move of versions carried is taken, and a ?v= the server read as no other version (the latest, or
// one gone) leaves the URL, as ui.html does
export function startFrame({
  boot = getBoot,
  versions = frameVersions,
  openPage = page,
  href = () => window.location.href,
  replace = (url) => {
    window.history.replaceState(window.history.state, '', url);
  },
}: Deps = {}): void {
  versions.arrive();
  const { version } = boot();
  if (openPage.diff() || !version || Number(version)) return;
  const url = new URL(href());
  url.searchParams.delete('v');
  replace(url.href);
}
