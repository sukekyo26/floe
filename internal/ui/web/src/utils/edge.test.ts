import { describe, expect, it } from 'vitest';

import { clampSide, edgeStep, shownSide, SIDE_MIN, sideMax } from '@/utils/edge';

describe(edgeStep, () => {
  it('moves an edge 16 px either way, to its bounds by Home and End, and takes no other key', () => {
    expect.assertions(5);
    expect(edgeStep('ArrowLeft', 200, 100, 400)).toBe(184);
    expect(edgeStep('ArrowRight', 200, 100, 400)).toBe(216);
    expect(edgeStep('Home', 200, 100, 400)).toBe(100);
    expect(edgeStep('End', 200, 100, 400)).toBe(400);
    expect(edgeStep('Enter', 200, 100, 400)).toBeNull();
  });
});

describe(clampSide, () => {
  it('keeps a side column from 160 px to half the window, up to 480 px, never under 160', () => {
    expect.assertions(2);
    expect([sideMax(2000), sideMax(701), sideMax(200)]).toStrictEqual([480, 350, SIDE_MIN]);
    expect([clampSide(100, 2000), clampSide(300.4, 2000), clampSide(900, 2000), clampSide(400, 701)]).toStrictEqual([
      160, 300, 480, 350,
    ]);
  });
});

describe(shownSide, () => {
  it('takes the width within bounds open, and none folded', () => {
    expect.assertions(1);
    expect([shownSide(true, 900, 2000), shownSide(false, 300, 2000)]).toStrictEqual([480, 0]);
  });
});
