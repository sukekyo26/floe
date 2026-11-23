import type { ReactNode } from 'react';
import { renderToString } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { Bar } from '@/app/bar';
import { testBoot } from '@/components/test-boot';
import { getBoot, setBoot } from '@/config/boot';

const none = () => undefined;
const draw = (search?: ReactNode) => {
  testBoot();
  setBoot({ ...getBoot(), screen: 'tasks' });
  // The server's state reads the address bar's host as it draws: node has no window
  vi.stubGlobal('window', { location: { host: '127.0.0.1:7325' } });
  return renderToString(
    <Bar tasks={search === undefined} search={search} onFind={none} onHelp={none} onSettings={none} />,
  );
};

describe(Bar, () => {
  it('holds the logo, the server, the screens and, at its end, the new pages and the three ways, by the marks the tests read', () => {
    expect.assertions(3);
    const html = draw();
    expect(html).toMatch(
      /data-testid="bar".*data-testid="bar-inner".*data-testid="brand".*id="server".*id="screens".*data-testid="bar-end".*id="news"/s,
    );
    expect(html).toMatch(
      /id="find-open"[^>]*aria-label="ui_find:.*id="keys-open"[^>]*aria-label="ui_help:.*id="settings-open"[^>]*aria-label="ui_settings:/s,
    );
    expect(html).not.toMatch(/id="(search|q|back)"/);
  });

  it('puts the list’s search between the screens and the end, with no way to the search modal beside it', () => {
    expect.assertions(2);
    const html = draw(<span id="search" />);
    expect(html).toMatch(/id="screens".*id="search".*data-testid="bar-end"/s);
    expect(html).not.toContain('id="find-open"');
  });
});
