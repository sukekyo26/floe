import { describe, expect, it, vi } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { ago, fmt } from '@/lib/time';

describe(ago, () => {
  it('says how long ago in its largest unit, just now under a minute, and nothing of what is no time', () => {
    expect.assertions(4);
    setBoot({ lang: 'en', messages: { ui_just_now: 'just now' } } as unknown as Boot);
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-11-21T12:00:00Z'));
    expect(ago('2026-11-21T11:59:30Z')).toBe('just now');
    expect(ago('2026-11-21T11:00:00Z')).toBe('1 hour ago');
    expect(ago('2026-11-19T12:00:00Z')).toBe('2 days ago');
    expect(ago('')).toBe('');
    vi.useRealTimers();
  });

  it('writes nothing of what is no time, as fmt is drawn into a row of none', () => {
    expect.assertions(2);
    setBoot({ lang: 'en', messages: {} } as unknown as Boot);
    expect(fmt('')).toBe('');
    expect(fmt('2026-11-21T12:00:00Z')).not.toBe('');
  });
});
