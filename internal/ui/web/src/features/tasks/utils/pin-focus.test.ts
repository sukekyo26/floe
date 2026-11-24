import { describe, expect, it } from 'vitest';

import { pinFocus, pinsSaid } from '@/features/tasks/utils/pin-focus';
import type { Page } from '@/types/page';

const page = (slug: string, title: string): Page => ({
  slug,
  title,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: 'p',
});

describe(pinFocus, () => {
  it('keeps the focus on the same part of a chip that stays', () => {
    expect.assertions(1);
    expect(pinFocus(['a', 'b'], ['b', 'a'], { slug: 'a', on: 'pin-off' })).toStrictEqual({ slug: 'a', on: 'pin-off' });
  });

  it('gives a chip gone’s focus to the title of the one in its place, or the last, or the help', () => {
    expect.assertions(4);
    expect(pinFocus(['a', 'b', 'c'], ['a', 'c'], { slug: 'b', on: 'pin-off' })).toStrictEqual({ slug: 'c', on: 'pin' });
    expect(pinFocus(['a', 'b'], ['a'], { slug: 'b', on: 'pin-off' })).toStrictEqual({ slug: 'a', on: 'pin' });
    expect(pinFocus(['a'], [], { slug: 'a', on: 'pin' })).toBe('help');
    expect(pinFocus([], ['b'], { slug: 'a', on: 'pin-off' })).toStrictEqual({ slug: 'b', on: 'pin' });
  });
});

describe(pinsSaid, () => {
  it('changes as any pin’s title, unread or folder does, not with what else a page has', () => {
    expect.assertions(2);
    const now = pinsSaid([page('a', 'A'), page('b', 'B')]);
    expect(pinsSaid([page('a', 'A'), page('b', 'B2')])).not.toBe(now);
    expect(pinsSaid([page('a', 'A'), { ...page('b', 'B'), description: 'more' }])).toBe(now);
  });
});
