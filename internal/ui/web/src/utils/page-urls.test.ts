import { describe, expect, it } from 'vitest';

import { diffHref, downloadHref, pageHref, rawHref, taskHref } from '@/utils/page-urls';

describe(pageHref, () => {
  it("writes a page's URLs", () => {
    expect.assertions(5);
    expect([
      pageHref({ slug: 'a' }),
      pageHref({ slug: 'a', v: 2 }),
      pageHref({ slug: 'a', file: 'x y.html' }),
    ]).toStrictEqual(['/a/a/', '/a/a/?v=2', '/a/a/?file=x%20y.html']);
    expect([
      rawHref({ slug: 'a' }),
      rawHref({ slug: 'a', v: 2, from: 1, key: 'k&' }),
      rawHref({ slug: 'a', file: 'd/x y.html' }),
    ]).toStrictEqual(['/a/a/?raw', '/a/a/?raw&v=2&from=1&key=k%26', '/a/a/d/x%20y.html']);
    expect([diffHref('a'), diffHref('a', 1, 3)]).toStrictEqual(['/a/a/?diff', '/a/a/?diff&from=1&to=3']);
    expect(downloadHref({ slug: 'a', v: 2 })).toBe('/api/pages/a/download?v=2');
    expect(taskHref({ project: 'p/q', name: 'S 1' })).toBe('/tasks?project=p%2Fq&task=S+1');
  });
});
