// The page's own task, as meta.json has it
export interface PageTask {
  name?: string;
  category?: string;
  pin?: boolean;
}

// An entry of GET /api/pages: a stored page, a link or a folder's link (store.Listed)
export interface Page {
  slug: string;
  title?: string;
  description: string;
  path?: string;
  template?: string;
  project?: string;
  favorite?: boolean;
  folder?: boolean;
  archived?: boolean;
  tags?: string[];
  task?: PageTask;
  createdAt: string;
  updatedAt: string;
  taskProject: string;
  news?: string;
  version?: number;
  versionCount?: number;
  missing?: boolean;
  themed?: boolean;
  // pickedTheme is the theme picked for the page on the management page, over the one it was drawn with
  pickedTheme?: string;
}

// A directory of a folder, GET /api/pages/{slug}/files (link.Listing)
export interface Listing {
  dirs: string[];
  files: { file: string; updatedAt: string }[];
  more: number;
}

// The version read of a page, as the server made its mark when it answered with the page
export interface ReadMark {
  n: number;
  at: string;
}
