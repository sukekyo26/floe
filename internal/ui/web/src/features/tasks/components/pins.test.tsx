import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { Pins } from '@/features/tasks/components/pins';
import type { Page } from '@/types/page';

const page = (slug: string, title: string, news?: string): Page => ({
  slug,
  title,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: 'p',
  ...(news ? { news } : {}),
});

describe(Pins, () => {
  it('draws each pin, unread bold with its dot, with its ×, the help before them', () => {
    expect.assertions(3);
    testBoot();
    const html = renderToString(<Pins pinned={[page('a', 'A', 'page'), page('b', 'B')]} hidden={false} />);
    expect(html).toMatch(/id="pins-help"[^>]*popovertarget="pins-tip".*id="pin-list"/is);
    expect([...html.matchAll(/data-slug="([^"]+)"/g)].map((m) => m[1])).toStrictEqual(['a', 'b']);
    expect(html).toMatch(
      /data-testid="pin" data-unread="true" href="\/a\/a\/"[^>]*>.*data-testid="dot".*data-testid="pin-off"/s,
    );
  });

  it('says when none is pinned, and hides with no project', () => {
    expect.assertions(2);
    testBoot();
    expect(renderToString(<Pins pinned={[]} hidden={false} />)).toContain('data-testid="pins-none"');
    expect(renderToString(<Pins pinned={[]} hidden />)).toMatch(/<div id="pins" hidden=""/);
  });
});
