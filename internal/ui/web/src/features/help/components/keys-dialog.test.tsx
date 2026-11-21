import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { KeysDialog, type HelpScreen } from '@/features/help/components/keys-dialog';

const draw = (screen: HelpScreen, { diff = false, diffGo = false, mac = false } = {}) => {
  testBoot();
  return renderToString(
    <KeysDialog
      open={null}
      screen={screen}
      diff={diff}
      diffGo={diffGo}
      backWords=""
      mac={mac}
      onClosed={() => undefined}
    />,
  );
};
// hidden tells whether the first element matching the start of a tag is hidden
const hidden = (html: string, start: string) =>
  String(new RegExp(`${start}[^>]*>`).exec(html)?.[0]).includes('hidden=""');

describe(KeysDialog, () => {
  it('holds every part closed, hiding those of another screen', () => {
    expect.assertions(4);
    const list = draw('list');
    expect([hidden(list, '<div id="keys-list"'), hidden(list, '<div id="keys-frame"')]).toStrictEqual([false, true]);
    const frame = draw('frame');
    expect([hidden(frame, '<div id="keys-list"'), hidden(frame, '<div id="keys-frame"')]).toStrictEqual([true, false]);
    expect(hidden(draw('taskList'), '<section id="keys-row"')).toBe(true);
    expect(hidden(draw('taskPage'), '<dt class="select-key"')).toBe(true);
  });

  it('tells d only where it goes, the diff keys in a diff, h on a task page and ⌘ on a Mac', () => {
    expect.assertions(5);
    expect(hidden(draw('list'), '<dt data-testid="diff-go"')).toBe(true);
    expect(hidden(draw('list', { diffGo: true }), '<dt data-testid="diff-go"')).toBe(false);
    expect([
      hidden(draw('frame'), '<dt class="diff-key"'),
      hidden(draw('frame', { diff: true }), '<dt class="diff-key"'),
    ]).toStrictEqual([true, false]);
    expect(draw('taskPage')).toContain('ui_tasks_back');
    expect(draw('list', { mac: true })).toMatch(/data-testid="mod"[^>]*>⌘</);
  });
});
