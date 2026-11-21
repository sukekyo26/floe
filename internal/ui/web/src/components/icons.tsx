import type { ReactNode } from 'react';

// The marks the shared components draw, as ui.html draws them (24 by 24, in the colour of their words)
function Line({ children, width = 1.8 }: { readonly children: ReactNode; readonly width?: number }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={width}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

export const StarIcon = () => (
  <Line>
    <path d="M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9l-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z" />
  </Line>
);
export const ArchiveIcon = () => (
  <Line>
    <path d="M4 5h16v4H4zM5 9v10h14V9M10 13h4" />
  </Line>
);
export const DoneIcon = () => (
  <Line width={2}>
    <path d="m5 12.5 4.5 4.5L19 7.5" />
  </Line>
);
export const BackIcon = () => (
  <Line width={2}>
    <path d="m15 6-6 6 6 6" />
  </Line>
);
export const MoreIcon = () => (
  <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
    <circle cx="5" cy="12" r="1.6" />
    <circle cx="12" cy="12" r="1.6" />
    <circle cx="19" cy="12" r="1.6" />
  </svg>
);
export const PlusIcon = () => (
  <Line width={2}>
    <path d="M12 5v14M5 12h14" />
  </Line>
);
// A link's and a folder's mark at the head of their row (ui.html LINK_MARK and FOLDER_MARK)
export const LinkMark = () => (
  <Line width={2}>
    <path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" />
  </Line>
);
export const FolderMark = () => (
  <Line width={2}>
    <path d="M3 6.5A1.5 1.5 0 0 1 4.5 5H9l2 2.5h8.5A1.5 1.5 0 0 1 21 9v9.5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18.5Z" />
  </Line>
);
