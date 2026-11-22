import { t } from '@/lib/t';

// ArchivedBadge says a page of a row is archived, where archived pages show among the rest (a task's page, the search)
export function ArchivedBadge() {
  return (
    <span data-testid="badge" className="rounded-full border border-line bg-subtle px-1.25 text-label text-muted">
      {t('ui_archived_badge')}
    </span>
  );
}
