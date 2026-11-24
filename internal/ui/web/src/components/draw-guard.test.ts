import { describe, expect, it } from 'vitest';

import { Guard } from '@/components/draw-guard';

const props = (attempt: number) => ({ attempt, pages: null as never, children: null });

describe(Guard, () => {
  it('tries to draw again once, after the next read ended, and not before', () => {
    expect.assertions(4);
    expect(Guard.getDerivedStateFromError()).toStrictEqual({ failedAt: -1 });
    expect(Guard.getDerivedStateFromProps(props(3), { failedAt: -1 })).toStrictEqual({ failedAt: 3 });
    expect(Guard.getDerivedStateFromProps(props(3), { failedAt: 3 })).toBeNull();
    expect(Guard.getDerivedStateFromProps(props(4), { failedAt: 3 })).toStrictEqual({ failedAt: null });
  });
});
