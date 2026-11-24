import { getBoot, type Boot } from '@/config/boot';
import { openingProject, taskProjects } from '@/features/tasks/utils/task-list';
import { projectKept as kept0, type ProjectKept } from '@/features/tasks/stores/project-kept';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { tasksFrom } from '@/utils/tasks-of';

interface Deps {
  query?: ListQuery;
  pages?: Pages;
  kept?: ProjectKept;
  boot?: () => Boot;
}

// holdProject keeps the task list's project and its URL as the tasks are read: no project asked, the one it opens with
// is put in the list's query the first time one is found, and stays, so that a task come into progress later does not
// change the list under the viewer; the viewer's project is kept the first time the tasks are read, if it is one there
// is; and the URL and the tab's query are written each time they change with the tasks read. Unreadable, nothing is
// written: the screen shows what was kept, which may be gone. The one place that writes the task list's URL. It returns
// what stops it
export function holdProject({
  query = listQuery,
  pages = pagePages,
  kept = kept0,
  boot = getBoot,
}: Deps = {}): () => void {
  let keptOnce = false;
  const hold = () => {
    const { read, entries, tasks: taken } = pages.state.get();
    if (!read || taken.error) return;
    const tasks = tasksFrom(entries, taken.tasks);
    const { project } = query.query.get();
    if (!project) {
      const first = boot().states.find((s) => s.first)?.name ?? '';
      const opening = openingProject({ tasks, entries, kept: kept.kept(), failed: false, first, lang: boot().lang });
      // Put in the query, which tells this again, to go on from there
      if (opening) {
        query.filter({ project: opening });
        return;
      }
    }
    if (!keptOnce) {
      keptOnce = true;
      if (taskProjects(tasks, entries).has(project)) kept.keep(project);
    }
    query.sync();
  };
  hold();
  const stops = [query.query.subscribe(hold), pages.state.subscribe(hold)];
  return () => {
    for (const stop of stops) stop();
  };
}
