import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { firstChip, moreIn, Shelf } from '@/components/shelf';
import { Chip } from '@/components/shelf-chip';
import { ChipGroup } from '@/components/shelf-chips';
import { NarrowFilter } from '@/components/shelf-filter';
import { ShelfWhat } from '@/components/shelf-what';
import { testBoot } from '@/components/test-boot';

const none = () => undefined;
const tag = (html: string, start: string) => String(new RegExp(`${start}[^>]*>`).exec(html)?.[0]);

describe(Chip, () => {
  it('is pressed when picked, named by what it picks, its count and words named for the tests', () => {
    expect.assertions(2);
    const html = renderToString(
      <Chip label="floe" n={3} pressed dot="red" data={{ 'data-project': 'floe' }} onPick={none} />,
    );
    expect(tag(html, '<button')).toMatch(/data-testid="chip" title="floe" aria-pressed="true".*data-project="floe"/);
    expect(html).toMatch(/data-testid="name"[^>]*>floe<.*data-testid="n"[^>]*>3</);
  });
});

describe(ChipGroup, () => {
  it('is hidden as its caller says, its chips in a group of the heading’s name', () => {
    expect.assertions(2);
    const draw = (hidden: boolean) =>
      renderToString(
        <ChipGroup id="shelf-tags" boxId="tags" title="Tags" hidden={hidden}>
          x
        </ChipGroup>,
      );
    expect(tag(draw(true), '<div id="shelf-tags"')).toContain('hidden=""');
    expect(draw(false)).toMatch(
      /<div id="shelf-tags"><h2[^>]*>Tags<\/h2><div id="tags" role="group" aria-label="Tags"/,
    );
  });
});

describe(NarrowFilter, () => {
  it('is marked narrowed while picks narrow the list, and tinted in the task list’s project', () => {
    expect.assertions(2);
    const plain = tag(renderToString(<NarrowFilter words="w" n={0} tint={null} onOpen={none} />), '<button');
    expect(plain).not.toMatch(/ data-narrowed=| data-tint=/);
    const lit = renderToString(<NarrowFilter words="w" n={2} tint="red" onOpen={none} />);
    expect(tag(lit, '<button')).toMatch(/id="narrow-filter" data-narrowed="true" data-tint="true" style="--c:red"/);
  });

  it('always shows its words, cut short, and draws no count but keeps its box', () => {
    expect.assertions(3);
    const named = renderToString(<NarrowFilter words="shop" n={0} tint="red" named onOpen={none} />);
    expect(tag(named, '<span id="narrow-filter-name"')).toMatch(
      /class="truncate narrow:max-w-\[4.5rem\] narrow:min-w-\[2em\] /,
    );
    expect(named).toMatch(/<span id="narrow-filter-n" aria-hidden="true" class="hidden"><\/span>/);
    expect(
      tag(renderToString(<NarrowFilter words="w" n={0} tint={null} onOpen={none} />), '<span id="narrow-filter-name"'),
    ).toContain('tiny:sr-only');
  });
});

describe(ShelfWhat, () => {
  it('is hidden as its caller says, and tinted in the task list’s project', () => {
    expect.assertions(2);
    testBoot();
    expect(tag(renderToString(<ShelfWhat words="w" tint={null} hidden onShow={none} />), '<button')).toMatch(
      /id="shelf-what"[^>]*hidden=""(?![^>]*data-tint)/,
    );
    expect(tag(renderToString(<ShelfWhat words="w" tint="red" hidden={false} onShow={none} />), '<button')).toMatch(
      /data-tint="true" style="--c:red"/,
    );
  });
});

describe(Shelf, () => {
  it('draws the rest of the shelf in the shelf while its modal is closed, the modal named for the tests', () => {
    expect.assertions(3);
    testBoot();
    const html = renderToString(
      <Shelf views={<i />} narrow={{ words: 'w', n: 0, tint: null }} more={<b id="part" />} />,
    );
    expect(html).toMatch(
      /<div id="shelf-body"[^>]*><div data-testid="views"[^>]*><i><\/i><button[^>]*id="narrow-filter"/,
    );
    expect(html).toMatch(/<div id="shelf-more"[^>]*><b id="part"><\/b><\/div><\/div><\/nav>/);
    expect(html).toMatch(
      /<dialog[^>]*id="filter-dialog"[^>]*aria-labelledby="filter-title".*<fieldset id="filter-box"/,
    );
  });

  it('puts the rest in one place at a time, and focuses the first chip that shows as the modal opens', () => {
    expect.assertions(2);
    expect([moreIn(true), moreIn(false)]).toStrictEqual(['dialog', 'shelf']);
    const rows = [
      { name: 'hidden', shows: false, adds: false },
      { name: 'add', shows: true, adds: true },
      { name: 'chip', shows: true, adds: false },
    ];
    expect([
      firstChip(
        rows,
        (r) => r.shows,
        (r) => r.adds,
      )?.name,
      firstChip(
        rows.slice(0, 2),
        (r) => r.shows,
        (r) => r.adds,
      ),
    ]).toStrictEqual(['chip', undefined]);
  });
});
