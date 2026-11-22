import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { dayGroups, dayOf } from '@/features/page-list/utils/list-days';
import type { Page } from '@/types/page';

const now = new Date(2026, 10, 21, 12);
const iso = (y: number, m: number, d: number) => new Date(y, m, d, 9).toISOString();

describe(dayOf, () => {
  it('heads today, yesterday and the week, then the month, its year before this one', () => {
    expect.assertions(1);
    testBoot();
    expect([
      dayOf(iso(2026, 10, 21), now, 'en'),
      dayOf(iso(2026, 10, 20), now, 'en'),
      dayOf(iso(2026, 10, 14), now, 'en'),
      dayOf(iso(2026, 10, 13), now, 'en'),
      dayOf(iso(2025, 11, 1), now, 'en'),
    ]).toStrictEqual([
      'ui_day_today:undefined',
      'ui_day_yesterday:undefined',
      'ui_day_week:undefined',
      'November',
      'December 2025',
    ]);
  });

  it('puts a space after a number in Japanese, as the page’s other words have', () => {
    expect.assertions(1);
    expect(dayOf(iso(2025, 2, 1), now, 'ja')).toBe('2025 年3 月');
  });
});

describe(dayGroups, () => {
  it('parts the entries in their order at each change of heading, keyed by it', () => {
    expect.assertions(1);
    testBoot();
    const page = (slug: string, updatedAt: string) => ({ slug, updatedAt }) as Page;
    const groups = dayGroups(
      [page('a', iso(2026, 10, 21)), page('b', iso(2026, 10, 21)), page('c', iso(2026, 10, 20))],
      now,
      'en',
    );
    expect(groups.map((g) => [g.key, g.entries.map((a) => a.slug)])).toStrictEqual([
      ['group:ui_day_today:undefined', ['a', 'b']],
      ['group:ui_day_yesterday:undefined', ['c']],
    ]);
  });
});
