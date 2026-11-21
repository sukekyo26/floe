import { getBoot, type Boot } from '@/config/boot';
import { session, type Kept } from '@/lib/storage';

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
      let move: { slug?: unknown; focus?: unknown; kept?: unknown } | null;
      try {
        move = JSON.parse(storage.read(VERSION_MOVE) ?? 'null') as typeof move;
      } catch {
        // Something else than ours: left as it is
        return { focus: false, kept: false };
      }
      storage.remove(VERSION_MOVE);
      if (move?.slug !== boot().framed) return { focus: false, kept: false };
      return { focus: move.focus === true, kept: move.kept === true };
    },
  };
}

export const versionMove = createVersionMove();
