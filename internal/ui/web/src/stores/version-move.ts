import { getBoot, type Boot } from '@/config/boot';
import { readJSON, session, type Kept } from '@/lib/storage';

// What a move to another version of the frame's page carries over: the focus on the choice, and a page kept unread
const VERSION_MOVE = 'version.move';

export interface VersionMove {
  keep: (focus: boolean, kept: boolean) => void;
  // take is what the move to this frame's page carried, once; nothing for another page's
  take: () => { focus: boolean; kept: boolean };
}

interface Deps {
  boot?: () => Boot;
  storage?: Kept;
}

export function createVersionMove({ boot = getBoot, storage = session }: Deps = {}): VersionMove {
  return {
    keep: (focus, kept) => {
      storage.write(VERSION_MOVE, JSON.stringify({ slug: boot().framed, focus, kept }));
    },
    take: () => {
      const move = readJSON(storage, VERSION_MOVE) as { slug?: unknown; focus?: unknown; kept?: unknown } | null;
      storage.remove(VERSION_MOVE);
      const here = move?.slug === boot().framed;
      return { focus: here && move.focus === true, kept: here && move.kept === true };
    },
  };
}

export const versionMove = createVersionMove();
