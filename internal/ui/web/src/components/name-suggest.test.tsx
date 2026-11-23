import type { FocusEvent } from 'react';
import { renderToString } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { whileGiving } from '@/components/dialog';
import { useNameSuggest, type NameSuggest } from '@/components/name-suggest';
import { testBoot } from '@/components/test-boot';

// The field of suggestions asked for as TaskSetDialog asks: in the body of the part, outside any modal's Scope
const fieldOf = (offers: () => []): NameSuggest['field'] => {
  testBoot();
  let field: NameSuggest['field'] | undefined;
  function Part() {
    field = useNameSuggest({ id: 'f', offers, put: () => undefined }).field;
    return null;
  }
  renderToString(<Part />);
  if (!field) throw new Error('not drawn');
  return field;
};

const focused = { currentTarget: { value: '' } } as unknown as FocusEvent<HTMLInputElement>;

describe(useNameSuggest, () => {
  it('opens its names as its field is reached, not as a hold gives the focus back, though asked outside the modal', () => {
    expect.assertions(2);
    const offers = vi.fn<() => []>(() => []);
    const field = fieldOf(offers);
    whileGiving({
      focus: () => {
        field.onFocus(focused);
      },
    });
    expect(offers).not.toHaveBeenCalled();
    field.onFocus(focused);
    expect(offers).toHaveBeenCalledTimes(1);
  });
});
