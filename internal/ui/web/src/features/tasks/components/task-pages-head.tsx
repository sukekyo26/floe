import { Button } from '@/components/button';
import { focusOn } from '@/components/focus-places';
import { PlusIcon } from '@/components/icons';
import { PartHead } from '@/features/tasks/components/part-head';
import { t } from '@/lib/t';
import type { FocusBack } from '@/types/focus';
import type { TaskOf } from '@/types/task';

// TaskPagesHead heads a task's pages, off #list: how many, as its row of the task list counts them (the archived left
// out), none said without a number as the family card's children are, and the button that adds one
export function TaskPagesHead({
  task,
  shown,
  onAddPage,
}: {
  readonly task: TaskOf | null;
  readonly shown: boolean;
  readonly onAddPage: (task: TaskOf, back: FocusBack) => void;
}) {
  const n = task?.pages.length ?? 0;
  return (
    <PartHead
      level={2}
      id="task-pages-head"
      hidden={!shown}
      place="px-4 pt-2.5"
      words={n > 0 ? t('ui_task_pages_head', n) : t('ui_task_pages_word')}
    >
      <Button
        id="task-add-page"
        adds
        label={t('ui_page_add')}
        onClick={() => {
          if (task) onAddPage(task, () => focusOn(document.getElementById('task-add-page')));
        }}
      >
        <PlusIcon />
        {t('ui_part_add')}
      </Button>
    </PartHead>
  );
}
