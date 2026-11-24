import type { ReactNode } from 'react';

// The marks the shared components draw (24 by 24, in the colour of their words)
// A mark's own name and its hidden, where a test reads which mark shows (the settings' head)
export interface IconProps {
  readonly testid?: string;
  readonly hidden?: boolean;
}

function Line({
  children,
  width = 1.8,
  testid,
  hidden,
}: { readonly children: ReactNode; readonly width?: number } & IconProps) {
  return (
    <svg
      data-testid={testid}
      // An svg has no hidden in React's types, which the attribute has: given as it is
      {...(hidden ? ({ hidden: true } as object) : {})}
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
// A stored page's mark among the shelf's kinds
export const PageMark = () => (
  <Line width={2}>
    <path d="M6 3h8l4 4v14H6zM14 3v4h4M9 12h6M9 16h6" />
  </Line>
);
// The star filled, of the shelf's view of favourites
export const StarMark = () => (
  <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
    <path d="M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9l-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z" />
  </svg>
);
// The shelf's way to its filters on a narrow screen
export const FilterIcon = () => (
  <Line width={2}>
    <path d="M4 6h16M7 12h10M10 18h4" />
  </Line>
);
// The search's mark, of the list's search and of the search over every page
export const SearchIcon = () => (
  <Line width={2}>
    <circle cx="11" cy="11" r="7" />
    <path d="m20 20-3.5-3.5" />
  </Line>
);
// A link's and a folder's mark at the head of their row
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
export const UnreadIcon = () => (
  <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
    <circle cx="12" cy="12" r="4.5" />
  </svg>
);
export const SelectIcon = () => (
  <Line>
    <rect x="4" y="4" width="16" height="16" rx="3" />
    <path d="m8.5 12 2.5 2.5 4.5-5" />
  </Line>
);
export const TagIcon = () => (
  <Line>
    <path d="M3 12V4h8l9 9-8 8-9-9Z" />
    <circle cx="7.5" cy="8.5" r="1.2" />
  </Line>
);
export const ThemeIcon = (props: IconProps) => (
  <Line {...props}>
    <path d="M12 3a9 9 0 1 0 0 18c.9 0 1.6-.7 1.6-1.6 0-.4-.2-.8-.4-1.1-.3-.3-.4-.7-.4-1.1 0-.9.7-1.6 1.6-1.6H16a5 5 0 0 0 5-5c0-4.1-4-7.6-9-7.6Z" />
    <circle cx="7.5" cy="10.5" r="1" />
    <circle cx="10.5" cy="7" r="1" />
    <circle cx="15" cy="7.5" r="1" />
  </Line>
);
export const PinIcon = () => (
  <Line>
    <path d="M9 3h6l-1 7 4 4v2H6v-2l4-4zM12 16v5" />
  </Line>
);
export const TaskSetIcon = () => (
  <Line>
    <path d="M9 6h11M9 12h11M9 18h11M4 6h.01M4 12h.01M4 18h.01" />
  </Line>
);
export const TaskOutIcon = () => (
  <Line>
    <path d="M9 6h11M9 12h11M4 6h.01M4 12h.01M14 18h6" />
  </Line>
);
export const ToIcon = () => (
  <Line>
    <path d="M5 12h14m-6-6 6 6-6 6" />
  </Line>
);
export const EditIcon = () => (
  <Line>
    <path d="M4 20h4L19 9a2.8 2.8 0 0 0-4-4L4 16v4Zm9.5-13.5 4 4" />
  </Line>
);
export const CopyIcon = () => (
  <Line>
    <path d="M9 9h10v11H9zM5 15V4h10" />
  </Line>
);
export const OpenIcon = () => (
  <Line>
    <path d="M14 4h6v6m0-6-8.5 8.5M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5" />
  </Line>
);
export const DownloadIcon = () => (
  <Line>
    <path d="M12 4v11m0 0-4.5-4.5M12 15l4.5-4.5M5 19h14" />
  </Line>
);
export const DeleteIcon = () => (
  <Line>
    <path d="M4 7h16M10 11v6m4-6v6M6 7l1 12h10l1-12M9 7V4h6v3" />
  </Line>
);
export const DiffIcon = () => (
  <Line>
    <path d="M3 4.5h7.5v15H3zM13.5 4.5H21v15h-7.5zM5.5 9h2.5M5.5 15h2.5M16 9h2.5M16 15h2.5" />
  </Line>
);
export const CloseIcon = () => (
  <Line width={2}>
    <path d="M6 6l12 12M18 6 6 18" />
  </Line>
);
export const HelpIcon = () => (
  <Line>
    <circle cx="12" cy="12" r="9" />
    <path d="M9.6 9.4a2.5 2.5 0 0 1 4.8.8c0 1.7-2.4 2-2.4 3.6" />
    <path d="M12 17.2v.01" />
  </Line>
);
export const ChevronIcon = () => (
  <Line width={2}>
    <path d="m9 6 6 6-6 6" />
  </Line>
);
export const SettingsIcon = (props: IconProps) => (
  <Line {...props}>
    <circle cx="12" cy="12" r="3" />
    <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1Z" />
  </Line>
);
export const TaskModeIcon = () => (
  <Line>
    <path d="M9 6h11M9 12h11M9 18h11" />
    <path d="m3 6 1.5 1.5L7 5M3 12l1.5 1.5L7 11M3 18l1.5 1.5L7 17" />
  </Line>
);
export const BackChevronIcon = () => (
  <Line>
    <path d="m15 6-6 6 6 6" />
  </Line>
);
// A directory and a page of a folder's tree
export const TreeDirIcon = () => (
  <Line>
    <path d="M3 6.5A1.5 1.5 0 0 1 4.5 5H9l2 2.5h8.5A1.5 1.5 0 0 1 21 9v9.5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18.5Z" />
  </Line>
);
export const TreePageIcon = () => (
  <Line>
    <path d="M7 3h7l5 5v12a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Zm7 0v5h5" />
  </Line>
);
// A column at the side, folded or open (the diff's contents)
export const SideIcon = () => (
  <Line>
    <rect x="3" y="4" width="18" height="16" rx="2" />
    <path d="M9 4v16" />
  </Line>
);
// The two columns left of the page, the one filled in: the shelf (ShelfColumnIcon) or the list (ListColumnIcon)
const TwoColumns = ({ children }: { readonly children: ReactNode }) => (
  <Line>
    <rect x="3" y="4" width="18" height="16" rx="2" />
    <path d="M9 4v16M15 4v16" />
    {children}
  </Line>
);
export const ShelfColumnIcon = () => (
  <TwoColumns>
    <path fill="currentColor" stroke="none" d="M3 6a2 2 0 0 1 2-2h4v16H5a2 2 0 0 1-2-2Z" />
  </TwoColumns>
);
export const ListColumnIcon = () => (
  <TwoColumns>
    <path fill="currentColor" stroke="none" d="M9 4h6v16H9Z" />
  </TwoColumns>
);
// A task's family card's parts and the way off it
export const ParentIcon = () => (
  <Line width={2}>
    <path d="m14 9-5-5-5 5" />
    <path d="M20 20h-7a4 4 0 0 1-4-4V4" />
  </Line>
);
export const KidsIcon = () => (
  <Line width={2}>
    <path d="m15 10 5 5-5 5" />
    <path d="M4 4v7a4 4 0 0 0 4 4h12" />
  </Line>
);
export const UnlinkIcon = () => (
  <Line width={2}>
    <path d="m18.84 12.25 1.72-1.71a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
    <path d="m5.17 11.75-1.71 1.71a5 5 0 0 0 7.07 7.07l1.71-1.71" />
    <path d="M8 2v3M2 8h3M16 22v-3M22 16h-3" />
  </Line>
);

// The buttons of a row of the task edit's lists, in lines: up, down, and the x that takes a URL or a category off, the
// one x of both as both are the same act
export const RowUpIcon = () => (
  <Line>
    <path d="M12 19V5" />
    <path d="m5 12 7-7 7 7" />
  </Line>
);

export const RowDownIcon = () => (
  <Line>
    <path d="M12 5v14" />
    <path d="m19 12-7 7-7-7" />
  </Line>
);

export const RowDropIcon = () => (
  <Line>
    <path d="M6 6l12 12M18 6 6 18" />
  </Line>
);

// The grip a row of the task edit's lists is dragged by
export const GripIcon = () => (
  <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
    <circle cx="9" cy="6" r="1.5" />
    <circle cx="15" cy="6" r="1.5" />
    <circle cx="9" cy="12" r="1.5" />
    <circle cx="15" cy="12" r="1.5" />
    <circle cx="9" cy="18" r="1.5" />
    <circle cx="15" cy="18" r="1.5" />
  </svg>
);
