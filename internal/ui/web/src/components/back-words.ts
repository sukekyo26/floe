import { t } from '@/lib/t';
import type { Back } from '@/utils/back';

// backWords says in full where a page whole goes back to: its way back's name and the help's h
export const backWords = (back: Back): string => {
  if (back.to === 'pages') return t('ui_to_page_list');
  if (back.to === 'tasks') return t('ui_tasks_back');
  return t('ui_back_to_task', back.task);
};
