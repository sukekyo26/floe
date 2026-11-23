import { describe, expect, it } from 'vitest';

import { startFrame } from '@/app/start';
import type { Boot } from '@/config/boot';

function start(version: string, search: string) {
  const replaced: string[] = [];
  let arrived = 0;
  startFrame({
    boot: () => ({ version }) as Boot,
    versions: {
      arrive: () => {
        arrived++;
      },
    },
    openPage: { diff: () => new URLSearchParams(search).has('diff') },
    href: () => `http://floe.test/a/p/${search}#part`,
    replace: (url) => replaced.push(url),
  });
  return { replaced, arrived };
}

describe(startFrame, () => {
  it('takes what a move of versions carried, once', () => {
    expect.assertions(1);
    expect(start('', '').arrived).toBe(1);
  });

  it('takes ?v= off the URL where the server read no other version: the latest or one gone', () => {
    expect.assertions(2);
    expect(start('0', '?v=3').replaced).toStrictEqual(['http://floe.test/a/p/#part']);
    expect(start('gone', '?v=9&file=x').replaced).toStrictEqual(['http://floe.test/a/p/?file=x#part']);
  });

  it('leaves the URL of an older version, of none asked, and of a diff', () => {
    expect.assertions(3);
    expect(start('2', '?v=2').replaced).toStrictEqual([]);
    expect(start('', '').replaced).toStrictEqual([]);
    expect(start('0', '?diff&v=1').replaced).toStrictEqual([]);
  });
});
