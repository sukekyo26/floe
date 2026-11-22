import { describe, expect, it } from 'vitest';

import type { Page } from '@/types/page';
import { enterOpens, nextRow, orderOf, rowOpens, rowPress } from '@/utils/row-press';

const press = { button: 0, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false };
const row = { picked: false, hasHref: true, menuOpen: false, selecting: false, oneColumn: false };
const page = (slug: string, change: Partial<Page> = {}): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...change,
});

describe(rowPress, () => {
  it('leaves a modified press to the link, goes whole on the row open pressed again, else beside or whole in one column', () => {
    expect.assertions(1);
    expect([
      rowPress({ ...press, ctrlKey: true }, row),
      rowPress({ ...press, button: 1 }, { ...row, picked: true }),
      rowPress(press, { ...row, picked: true }),
      rowPress(press, row),
      rowPress(press, { ...row, oneColumn: true }),
    ]).toStrictEqual(['link', 'link', 'again', 'beside', 'one-column']);
  });

  it('opens beside again with a menu open, rows selected, or a title that does not link', () => {
    expect.assertions(1);
    const picked = { ...row, picked: true };
    expect([
      rowPress(press, { ...picked, menuOpen: true }),
      rowPress(press, { ...picked, selecting: true }),
      rowPress(press, { ...picked, hasHref: false }),
      rowPress(press, { ...picked, hasHref: false, oneColumn: true }),
    ]).toStrictEqual(['beside', 'beside', 'beside', 'one-column']);
  });
});

describe(orderOf, () => {
  it('goes through the pages alone by j and k, and tells every row drawn as shown', () => {
    expect.assertions(1);
    expect(orderOf([page('a'), page('f', { folder: true }), page('g', { missing: true }), page('b')])).toStrictEqual({
      order: ['a', 'b'],
      shown: ['a', 'f', 'g', 'b'],
    });
  });
});

describe(rowOpens, () => {
  it('opens beside a page, a folder’s page of its tree, and nothing in one column, of a directory, a folder or no entry', () => {
    expect.assertions(1);
    const at = { oneColumn: false, page: page('a'), dir: false, file: '' };
    const folder = page('f', { folder: true });
    expect([
      rowOpens(at),
      rowOpens({ ...at, page: folder, file: 'x.html' }),
      rowOpens({ ...at, oneColumn: true }),
      rowOpens({ ...at, page: folder, dir: true }),
      rowOpens({ ...at, page: folder }),
      rowOpens({ ...at, page: undefined }),
    ]).toStrictEqual([true, true, false, false, false, false]);
  });
});

describe(enterOpens, () => {
  it('opens whole on a plain Enter, Shift too, and not with Ctrl, ⌘ or Alt, composing, or on a title without a link or with a tree', () => {
    expect.assertions(1);
    const key = { key: 'Enter', ctrlKey: false, metaKey: false, altKey: false };
    const title = { hasHref: true, expandable: false, composing: false };
    expect([
      enterOpens(key, title),
      enterOpens({ ...key, shiftKey: true } as typeof key, title),
      enterOpens({ ...key, ctrlKey: true }, title),
      enterOpens({ ...key, metaKey: true }, title),
      enterOpens({ ...key, altKey: true }, title),
      enterOpens(key, { ...title, composing: true }),
      enterOpens({ ...key, key: 'a' }, title),
      enterOpens(key, { ...title, hasHref: false }),
      enterOpens(key, { ...title, expandable: true }),
    ]).toStrictEqual([true, true, false, false, false, false, false, false, false]);
  });
});

describe(nextRow, () => {
  it('starts at the first down or the last up, and stops at either end', () => {
    expect.assertions(1);
    expect([
      nextRow(3, -1, 1),
      nextRow(3, -1, -1),
      nextRow(3, 0, 1),
      nextRow(3, 2, 1),
      nextRow(3, 0, -1),
    ]).toStrictEqual([0, 2, 1, 2, 0]);
  });
});
