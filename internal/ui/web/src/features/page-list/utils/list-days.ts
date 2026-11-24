import { t } from '@/lib/t';
import type { Page } from '@/types/page';

const DAY_MS = 86_400_000;
const dateOf = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();

// dayOf is the heading of the day iso falls on, from now: today, yesterday, this week, then its month, with its year
// before this one, in the browser's words of lang
export function dayOf(iso: string, now: Date, lang: string): string {
  const at = new Date(iso);
  const days = Math.round((dateOf(now) - dateOf(at)) / DAY_MS);
  if (days <= 0) return t('ui_day_today');
  if (days === 1) return t('ui_day_yesterday');
  if (days <= 7) return t('ui_day_week');
  const words = at.toLocaleDateString(
    lang,
    at.getFullYear() === now.getFullYear() ? { month: 'long' } : { year: 'numeric', month: 'long' },
  );
  // With Japanese's space after a number, as the page's other words have
  return words.replace(/(\d)([\u5e74\u6708])/g, '$1 $2');
}

export interface DayGroup {
  readonly key: string;
  readonly heading: string;
  readonly entries: Page[];
}

// dayGroups parts the entries, in their order, at each change of the day's heading
// LIMIT: a folder is placed by the time it was linked, under that day's heading, as it shows no time: what changes inside
// does not show
export function dayGroups(shown: readonly Page[], now: Date, lang: string): DayGroup[] {
  const days: DayGroup[] = [];
  for (const a of shown) {
    const heading = dayOf(a.updatedAt, now, lang);
    const last = days.at(-1);
    if (last?.heading === heading) last.entries.push(a);
    else days.push({ key: `group:${heading}`, heading, entries: [a] });
  }
  return days;
}
