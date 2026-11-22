import { createKeptSide } from '@/stores/kept-side';

// The frame's tree, of every folder: its width and whether it shows, kept apart from the diff's contents
export const frameSide = createKeptSide('frame.side', 280);
