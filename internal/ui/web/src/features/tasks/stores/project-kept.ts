import { local, type Kept } from '@/lib/storage';

// The viewer's last project of the task list, kept in the browser: whose it is, not the store's
const PROJECT_KEY = 'pages.taskProject';

export interface ProjectKept {
  // kept is the project kept, null for none
  kept: () => string | null;
  // keep is written as the list is first read and as a project is pressed, not at each look, which would take back
  // what another window chose
  keep: (name: string) => void;
}

export function createProjectKept(storage: Kept = local): ProjectKept {
  return {
    kept: () => storage.read(PROJECT_KEY),
    keep: (name) => {
      storage.write(PROJECT_KEY, name);
    },
  };
}

export const projectKept = createProjectKept();
