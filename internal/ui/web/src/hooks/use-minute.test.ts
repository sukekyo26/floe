import { describe, expect, it, vi } from 'vitest';

import { createMinute } from '@/hooks/use-minute';

describe(createMinute, () => {
  it('tells the time every minute while read, and stops with the last that reads it', () => {
    expect.assertions(4);
    vi.useFakeTimers();
    vi.setSystemTime(30_000);
    const minute = createMinute();
    expect(minute.get()).toBe(0);
    const told = vi.fn<() => void>();
    const stop = minute.subscribe(told);
    vi.advanceTimersByTime(60_000);
    expect([told.mock.calls.length, minute.get()]).toStrictEqual([1, 60_000]);
    stop();
    vi.advanceTimersByTime(120_000);
    expect(told).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBe(0);
    vi.useRealTimers();
  });
});
