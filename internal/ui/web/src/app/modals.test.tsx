import { describe, expect, it } from 'vitest';

import { diffGoes, freeOf } from '@/app/modals';
import { createStore } from '@/lib/store';
import type { OpenAt } from '@/stores/open-page';
import type { Page } from '@/types/page';

describe(freeOf, () => {
  it('is free with no modal open, or with only the one closing in the same press', () => {
    expect.assertions(5);
    expect(freeOf([])).toBe(true);
    expect(freeOf(['keys'])).toBe(false);
    expect(freeOf(['find'], 'find')).toBe(true);
    expect(freeOf(['find', 'keys'], 'find')).toBe(false);
    expect(freeOf(['keys'], 'find')).toBe(false);
  });
});

const page = (more: Partial<Page> = {}): Page => ({
  slug: 'p',
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  versionCount: 2,
  ...more,
});
const goes = (where: { frame?: OpenAt; picked?: OpenAt; diff?: boolean }, entries: Page[] = [page()]) =>
  diffGoes(
    {
      frameAt: () => where.frame ?? null,
      picked: createStore(where.picked ?? null),
      diff: () => where.diff ?? false,
    },
    entries,
  );

describe(diffGoes, () => {
  it('goes to a diff of the page open beside the list or whole, of two versions and more', () => {
    expect.assertions(4);
    const at = { slug: 'p', file: '', v: 0 };
    expect(goes({ picked: at })).toBe(true);
    expect(goes({ frame: at })).toBe(true);
    expect(goes({ frame: at }, [page({ versionCount: 1 })])).toBe(false);
    expect(goes({})).toBe(false);
  });

  it('does not in a diff, for a page whole not found, or for a folder’s file', () => {
    expect.assertions(4);
    const at = { slug: 'p', file: '', v: 0 };
    expect(goes({ frame: at, diff: true })).toBe(false);
    expect(goes({ frame: at }, [page({ missing: true })])).toBe(false);
    expect(goes({ frame: at }, [])).toBe(false);
    expect(goes({ frame: { ...at, file: 'a.html' } })).toBe(false);
  });
});
