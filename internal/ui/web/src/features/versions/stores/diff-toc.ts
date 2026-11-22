import { createKeptSide } from '@/stores/kept-side';

// The diff's contents: their width and whether they show, kept apart from the frame's tree (closing one does not close
// the other)
export const diffToc = createKeptSide('diff.toc', 240);
