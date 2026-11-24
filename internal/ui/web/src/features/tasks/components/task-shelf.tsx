import type { ReactNode } from 'react';

import { ArchiveIcon } from '@/components/icons';
import { Shelf } from '@/components/shelf';
import { ShelfAdd, ShelfAddButton } from '@/components/shelf-add';
import { Chip, SHELF_MARK } from '@/components/shelf-chip';
import { ChipGroup } from '@/components/shelf-chips';
import { ViewButton } from '@/components/shelf-views';
import { getBoot } from '@/config/boot';
import { projectKept as kept0, type ProjectKept } from '@/features/tasks/stores/project-kept';
import { stateWord } from '@/features/tasks/utils/kin-forms';
import { projectChips, projectPress, stateCounts, taskProjects, taskRanks } from '@/features/tasks/utils/task-list';
import { projectWord, t } from '@/lib/t';
import type { TaskOf } from '@/types/task';
import type { Page } from '@/types/page';
import { projectColor } from '@/utils/project-color';
import { listQuery, type ListQuery } from '@/stores/list-query';

// The dot of a state's view, as its tasks' state is coloured; gone in the row of pills
const DOT: Record<string, string> = { doing: 'bg-chg', todo: 'bg-faint', done: 'bg-ins' };

export interface TaskShelfProps {
  readonly tasks: readonly TaskOf[];
  readonly entries: readonly Page[];
  // project and view are those the list shows; failed tells the tasks unreadable
  readonly project: string;
  readonly view: string;
  readonly failed: boolean;
  readonly addHidden: boolean;
  readonly onAdd: () => void;
  readonly foot?: ReactNode;
  readonly query?: ListQuery;
  readonly kept?: ProjectKept;
}

// TaskShelf is the task list's shelf: the states as its views, each with the project's tasks of it, and the projects,
// one always shown. A press on what is shown already changes nothing; a project pressed is kept for the next opening,
// once the tasks are read
export function TaskShelf({
  tasks,
  entries,
  project,
  view,
  failed,
  addHidden,
  onAdd,
  foot,
  query = listQuery,
  kept = kept0,
}: TaskShelfProps) {
  const states = getBoot().states;
  const counts = stateCounts(tasks, project, states);
  const ranks = taskRanks(entries, taskProjects(tasks, entries));
  const tint = project ? projectColor(project, ranks) : null;
  const pickView = (name: string) => {
    if (name !== view) query.filter({ view: name });
  };
  const pickProject = (name: string) => {
    const press = projectPress(name, project, failed);
    if (press.keep) kept.keep(name);
    if (press.show) query.filter({ project: name });
  };
  const views = (
    <>
      {states
        .filter((s) => s.name !== 'archived')
        .map((s) => (
          <ViewButton
            key={s.name}
            view={s.name}
            mark={
              <span
                className={`mx-[calc((var(--spacing-shelf-mark)-8px)/2)] size-2 flex-none rounded-full wide:hidden ${DOT[s.name] ?? ''}`}
              />
            }
            label={stateWord(s.name)}
            n={counts[s.name] ?? 0}
            lit={view === s.name}
            onPress={() => {
              pickView(s.name);
            }}
          />
        ))}
      <ViewButton
        view="archived"
        mark={<ArchiveIcon />}
        markColor={`${SHELF_MARK} wide:aria-pressed:[&_svg]:text-on-ink`}
        label={stateWord('archived')}
        labelFrom="narrow"
        countFolds
        n={counts['archived'] ?? 0}
        lit={view === 'archived'}
        onPress={() => {
          pickView('archived');
        }}
      />
    </>
  );
  const more = (
    <>
      <ChipGroup id="shelf-projects" boxId="chips" title={t('ui_project')} hidden={false}>
        {projectChips(tasks, entries, project, view).map(({ name, n }) => (
          <Chip
            key={name}
            label={projectWord(name)}
            n={n}
            pressed={name === project}
            dot={projectColor(name, ranks)}
            data={{ 'data-project': name }}
            onPick={() => {
              pickProject(name);
            }}
          />
        ))}
      </ChipGroup>
      {/* Under what it adds to: the projects */}
      <ShelfAdd hidden={addHidden}>
        <ShelfAddButton
          id="add-task"
          short={t('ui_task_add_short')}
          label={t('ui_task_add')}
          hidden={addHidden}
          onAdd={onAdd}
        />
      </ShelfAdd>
    </>
  );
  return (
    <Shelf
      views={views}
      narrow={{ words: project ? projectWord(project) : t('ui_narrow'), n: 0, tint, named: true }}
      more={more}
      {...(foot === undefined ? {} : { foot })}
    />
  );
}
