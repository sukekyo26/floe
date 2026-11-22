import { getBoot, type Boot } from '@/config/boot';
import { createStore, type Store } from '@/lib/store';

// A page open: its slug, a folder's page's file ('' for any other) and, of a stored page, a version other than the latest
export interface OpenAt {
  slug: string;
  file: string;
  v: number;
}

export interface OpenPage {
  // picked is the row whose page is open beside the list, null for none
  picked: Store<OpenAt | null>;
  // frameAt is what the frame shows, null on the lists. The server read ?v= (a version, 0 for the latest, gone for
  // none); a diff loads no page
  frameAt: () => OpenAt | null;
  // diff tells the frame shows the diff of the page's versions (?diff), which loads no page
  diff: () => boolean;
  isPicked: (x: { slug: string; file?: string }) => boolean;
  // shownSlug is the page the frame shows, whole or beside the list
  shownSlug: () => string | undefined;
}

interface Deps {
  boot?: () => Boot;
  search?: () => string;
}

export function createOpenPage({ boot = getBoot, search = () => window.location.search }: Deps = {}): OpenPage {
  const picked = createStore<OpenAt | null>(null);
  const diff = (): boolean => {
    if (boot().screen !== 'frame') return false;
    const query = new URLSearchParams(search());
    return !query.has('file') && query.has('diff');
  };
  // framed is '' on the lists, which are no frame
  const frameAt = (): OpenAt | null => {
    const { screen, framed, version } = boot();
    if (screen !== 'frame') return null;
    const file = new URLSearchParams(search()).get('file') ?? '';
    return { slug: framed, file, v: diff() ? 0 : Number(version) || 0 };
  };
  return {
    picked,
    frameAt,
    diff,
    isPicked: (x) => {
      const at = picked.get();
      return at !== null && x.slug === at.slug && (x.file ?? '') === at.file;
    },
    shownSlug: () => frameAt()?.slug ?? picked.get()?.slug,
  };
}

export const openPage = createOpenPage();
