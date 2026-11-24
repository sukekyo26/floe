import { Badge } from '@/components/badge';
import { t } from '@/lib/t';

// ArchivedBadge says a page of a row is archived, where archived pages show among the rest (a task's page, the search)
export function ArchivedBadge() {
  return <Badge>{t('ui_archived_badge')}</Badge>;
}
