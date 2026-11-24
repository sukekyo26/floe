import { describe, expect, it } from 'vitest';

import { nameOffers, suggestKey } from '@/utils/name-offers';

const NAMES = ['#12', '#120', 'a#1', 'other'];
const words = (written: string, typed = false) => nameOffers(NAMES, written, typed).map((x) => x.name);

describe(nameOffers, () => {
  it('offers the names that hold what is written, those that start with it first', () => {
    expect.assertions(2);
    expect(words('#1')).toStrictEqual(['#12', '#120', 'a#1']);
    expect(words('')).toStrictEqual(NAMES);
  });

  it('offers them all for a name written whole, unless just typed whole with none going on from it', () => {
    expect.assertions(3);
    expect(words('other')).toStrictEqual(NAMES);
    expect(words('other', true)).toStrictEqual([]);
    expect(words('#12', true)).toStrictEqual(NAMES);
  });

  it('offers a name of none last, shown as fresh says, and the rest as detail says', () => {
    expect.assertions(2);
    const ways = {
      detail: (name: string) => ({ name, label: `${name}!` }),
      fresh: (name: string) => ({ name, label: `new ${name}` }),
    };
    expect(nameOffers(NAMES, ' #9 ', false, ways)).toStrictEqual([{ name: '#9', label: 'new #9' }]);
    expect(nameOffers(NAMES, 'oth', false, ways)).toStrictEqual([
      { name: 'other', label: 'other!' },
      { name: 'oth', label: 'new oth' },
    ]);
  });
});

describe(suggestKey, () => {
  it('opens the names on ↓ or ↑, and reaches the next of them round the ends once shown', () => {
    expect.assertions(4);
    expect(suggestKey('ArrowDown', false, -1, 3)).toStrictEqual({ act: 'open' });
    expect(suggestKey('ArrowDown', true, -1, 3)).toStrictEqual({ act: 'reach', at: 0 });
    expect(suggestKey('ArrowUp', true, 0, 3)).toStrictEqual({ act: 'reach', at: 2 });
    expect(suggestKey('ArrowDown', true, -1, 0)).toBeNull();
  });

  it('puts in the one reached on Enter, and closes the names on Escape and Tab alone while shown', () => {
    expect.assertions(5);
    expect(suggestKey('Enter', true, 1, 3)).toStrictEqual({ act: 'put', at: 1 });
    expect(suggestKey('Enter', true, -1, 3)).toBeNull();
    expect(suggestKey('Escape', true, -1, 3)).toStrictEqual({ act: 'close' });
    // Shut, Escape is the modal's
    expect(suggestKey('Escape', false, -1, 3)).toBeNull();
    expect(suggestKey('Tab', false, -1, 0)).toStrictEqual({ act: 'close' });
  });
});
