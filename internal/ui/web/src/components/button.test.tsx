import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { Button } from '@/components/button';

describe(Button, () => {
  it('is a plain button unless told to submit, in one size and one kind', () => {
    expect.assertions(4);
    expect(renderToString(<Button>Close</Button>)).toContain('type="button"');
    const foot = renderToString(
      <Button size="foot" kind="primary" closes>
        Save
      </Button>,
    );
    expect(foot).toMatch(/h-9(?![^"]*h-8)/);
    expect(foot).toContain('bg-ink');
    expect(foot).toContain('data-close=""');
  });

  it('is quiet with no ground or line, red in its words as a delete, and says it waits as armed, its room the same', () => {
    expect.assertions(4);
    const classes = (html: string) => String(/class="([^"]*)"/.exec(html)?.[1]).split(' ');
    const plain = classes(renderToString(<Button>Tags</Button>));
    const quiet = classes(renderToString(<Button quiet>Tags</Button>));
    expect([
      quiet.includes('bg-transparent'),
      quiet.includes('border-transparent'),
      plain.includes('bg-panel'),
    ]).toStrictEqual([true, true, true]);
    const room = (all: string[]) => all.filter((c) => /^(px|h)-/.test(c));
    expect(room(quiet)).toStrictEqual(room(plain));
    expect(classes(renderToString(<Button tone="danger">Delete</Button>))).toContain('text-danger');
    expect(renderToString(<Button armed>Delete 2?</Button>)).toContain('data-armed="true"');
  });
});
