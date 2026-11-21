import { describe, expect, it } from 'vitest';

import { edgeStep } from '@/utils/edge';

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
