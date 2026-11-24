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

  it('closes its names on the field’s own blur, a listener its ref puts on and takes off', () => {
    expect.assertions(3);
    const field = fieldOf(() => []);
    const on = vi.fn<(type: string, listener: () => void) => void>();
    const off = vi.fn<(type: string, listener: () => void) => void>();
    const input = { addEventListener: on, removeEventListener: off } as unknown as HTMLInputElement;
    const takeOff = field.ref(input);
    expect(on).toHaveBeenCalledWith('blur', expect.any(Function));
    takeOff?.();
    expect(off).toHaveBeenCalledWith('blur', on.mock.calls[0]?.[1]);
    expect(field).not.toHaveProperty('onBlur');
  });
});
