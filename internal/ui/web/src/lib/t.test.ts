import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { projectWord, t } from '@/lib/t';

const made = () => {
  setBoot({
    messages: { ui_http_error: '%[2]s: %[1]s, %s %d%%', ui_no_project: 'none' },
  } as unknown as Boot);
};

describe(t, () => {
  it('fills %s, %d, %[n]s, going on after the one named, and %%, and says a key the table lacks as itself', () => {
    expect.assertions(3);
    made();
    expect(t('ui_http_error', 'a', 'b')).toBe('b: a, b undefined%');
    expect(t('ui_add')).toBe('ui_add');
    expect(projectWord('/')).toBe('none');
  });
});
