import { describe, expect, it } from 'vitest';

import { columnWidths, listShown } from '@/utils/column-widths';

const none = { shelf: null, list: null };
const wide = { shelfShown: true, innerWidth: 1600, rem: 16 };

describe(columnWidths, () => {
  it('goes by the text of the browser for its defaults and bounds', () => {
    expect.assertions(2);
    expect(columnWidths(none, wide)).toStrictEqual({
      shelf: 200,
      list: 408,
      shelfMin: 160,
      shelfMax: 320,
      listMin: 320,
      listMax: 880,
    });
    const larger = columnWidths(none, { ...wide, rem: 20 });
    expect([larger.shelf, larger.list, larger.listMin]).toStrictEqual([250, 510, 400]);
  });

  it('holds what is kept within the bounds, rounded', () => {
    expect.assertions(2);
    const held = columnWidths({ shelf: 99_999, list: -5 }, wide);
    expect([held.shelf, held.list]).toStrictEqual([320, 320]);
    const odd = columnWidths({ shelf: 200.6, list: 400.4 }, wide);
    expect([odd.shelf, odd.list]).toStrictEqual([201, 400]);
  });

  it('gives the shelf nothing where it is not shown, the list the room it leaves', () => {
    expect.assertions(1);
    const folded = columnWidths(none, { ...wide, shelfShown: false });
    expect([folded.shelf, folded.listMax]).toStrictEqual([0, 1080]);
  });

  it('holds the list under what is kept in a narrower window, never under its least', () => {
    expect.assertions(2);
    const narrower = columnWidths({ shelf: 160, list: 740 }, { ...wide, innerWidth: 1200 });
    expect([narrower.listMax, narrower.list]).toStrictEqual([520, 520]);
    const narrow = columnWidths({ shelf: 160, list: 740 }, { ...wide, innerWidth: 600 });
    expect([narrow.listMax, narrow.list]).toStrictEqual([320, 320]);
  });
});

describe(listShown, () => {
  it('shows the list on the task list, as kept on the page list and a frame, on a task page until a page is picked', () => {
    expect.assertions(1);
    expect([
      listShown('taskList', false, true),
      listShown('list', false, false),
      listShown('list', true, true),
      listShown('frame', false, false),
      listShown('taskPage', false, false),
      listShown('taskPage', false, true),
      listShown('taskPage', true, true),
    ]).toStrictEqual([true, false, true, false, true, false, true]);
  });
});
