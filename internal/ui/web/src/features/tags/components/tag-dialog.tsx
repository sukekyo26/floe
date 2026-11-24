import { useEffect, useState } from 'react';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { TagIcon } from '@/components/icons';
import { getBoot } from '@/config/boot';
import { applyTags as pageApply, type TagApply } from '@/features/tags/stores/apply-tags';
import {
  changed,
  changes,
  chips,
  fresh,
  nextMark,
  startMarks,
  tagNames,
  taken,
  type Mark,
  type Marks,
} from '@/features/tags/utils/tag-marks';
import { useHold } from '@/hooks/use-hold';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import type { ActOn } from '@/stores/act-on';
import { pages as pagePages, type Pages } from '@/stores/pages';
import type { FocusBack } from '@/types/focus';
import type { Page } from '@/types/page';
import { composing } from '@/utils/keys';

// What the tag modal is opened for: the pages of slugs, its words by how many they are, and how it writes them
export interface TagTarget {
  slugs: string[];
  title: (n: number) => string;
  done: (n: number) => string;
  failed: (n: number, why: string) => string;
  // send and after are the selected's (P5); a page's own leave them out
  send?: ActOn;
  after?: TagApply['after'];
  // back is where the focus goes as it closes; none, where the browser takes it
  back: FocusBack | null;
}

// pageTags is the target of one page's tags, from its menu or its tag line
export const pageTags = (a: Page, back: FocusBack): TagTarget => ({
  slugs: [a.slug],
  title: () => t('ui_tag_one_title', a.title ?? a.slug),
  done: () => t('ui_tag_done', a.title ?? a.slug),
  failed: (_n, why) => t('ui_tag_failed', a.title ?? a.slug, why),
  back,
});

interface Opened {
  target: TagTarget;
  n: number;
  marks: Marks;
  want: Map<string, Mark>;
}

const MARK_WORD: Record<Mark, string> = { all: '✓', some: '−', none: '+' };
const CHIP: Record<Mark, string> = {
  all: 'border-ink bg-ink text-on-ink',
  some: 'border-dashed border-ink bg-sel text-fg',
  none: 'border-line bg-panel text-muted hover:border-faint',
};

// opening takes the marks of target's pages as listed now; none listed (gone since), it is not opened
function opening(target: TagTarget, entries: Page[], lang: string): Opened | null {
  const found = target.slugs.map((slug) => entries.find((a) => a.slug === slug)).filter((a) => a !== undefined);
  if (found.length === 0) return null;
  const marks = startMarks(found, tagNames(entries, lang));
  return { target, n: found.length, marks, want: new Map(marks.start) };
}

interface TagDialogProps {
  // target is the pages whose tags it changes, null while closed
  readonly target: TagTarget | null;
  readonly onClosed: () => void;
  readonly pages?: Pages;
  readonly apply?: (apply: TagApply) => Promise<void>;
}

// TagDialog is the one modal that changes tags (ui.md: openTags), of a page from its menu or its tag line, or of the
// selected: a chip each tag, which a press takes through its marks, and a field that finds a chip or makes a tag
export function TagDialog({ target, onClosed, pages = pagePages, apply = pageApply }: TagDialogProps) {
  const hold = useHold();
  const { entries } = useStore(pages.state);
  const lang = getBoot().lang;
  const [seen, setSeen] = useState<TagTarget | null>(null);
  const [opened, setOpened] = useState<Opened | null>(null);
  const [find, setFind] = useState('');
  // The way back stays as the modal closes: target is null by then
  const [back, setBack] = useState<FocusBack | null>(null);
  if (target !== seen) {
    setSeen(target);
    if (target) {
      const now = opening(target, entries, lang);
      setOpened(now);
      setFind('');
      setBack(() => target.back);
    }
  }
  // None of its pages listed any more (gone since): it is not opened, and its opener is told
  useEffect(() => {
    if (target && !opened) onClosed();
  }, [target, opened, onClosed]);
  const open = target !== null && opened !== null;
  const want = opened?.want ?? new Map<string, Mark>();
  const on = opened?.marks.on ?? new Map<string, number>();
  const start = opened?.marks.start ?? new Map<string, Mark>();
  const names = chips(want, on, find, lang);
  const setMark = (name: string, mark: Mark) => {
    if (opened) setOpened({ ...opened, want: new Map(opened.want).set(name, mark) });
  };
  // A tag taken or made is on all of the pages, whatever it had; the field is left for the next
  const take = (name: string) => {
    setMark(name, 'all');
    setFind('');
  };
  const make = () => {
    const name = find.trim();
    if (!fresh(want, name) || !opened) return;
    setOpened({
      ...opened,
      marks: {
        on: new Map(opened.marks.on).set(name, 0),
        start: new Map(opened.marks.start).set(name, 'none'),
      },
      want: new Map(opened.want).set(name, 'all'),
    });
    setFind('');
  };

  return (
    <Dialog
      id="tag-dialog"
      open={open}
      onClosed={onClosed}
      afterClose={() => back?.() ?? false}
      hold={hold}
      labelledBy="tag-title"
    >
      <form
        id="tag-form"
        className="grid gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (!opened) return;
          const { add, remove } = changes(start, want);
          const { target: of } = opened;
          void apply({
            slugs: of.slugs,
            add,
            remove,
            words: { done: of.done, failed: of.failed },
            ...(of.send ? { send: of.send } : {}),
            ...(of.after ? { after: of.after } : {}),
            whileHeld: hold.whileHeld,
            close: onClosed,
          });
        }}
      >
        <DialogHead icon={<TagIcon />} titleId="tag-title" title={opened ? opened.target.title(opened.n) : ''} />
        <input
          id="tag-find"
          data-testid="text"
          type="search"
          className="h-9 w-full min-w-0 rounded-small border border-line bg-panel px-3 text-fg shadow-sm placeholder:text-faint"
          aria-label={t('ui_tag_find')}
          placeholder={t('ui_tag_find')}
          aria-controls="tag-boxes"
          autoComplete="off"
          value={find}
          onChange={(e) => {
            setFind(e.target.value);
          }}
          // Enter takes the tag of the words, or the one chip left, as a field's suggestions did; else it makes the
          // words a tag. It never sends
          onKeyDown={(e) => {
            if (e.key !== 'Enter' || composing(e.nativeEvent)) return;
            e.preventDefault();
            const name = taken(names, find);
            if (name === null) make();
            else take(name);
          }}
        />
        <fieldset
          id="tag-boxes"
          aria-labelledby="tag-title"
          className="m-0 flex max-h-64 min-w-0 flex-wrap content-start gap-1.5 overflow-y-auto border-0 p-0.75"
        >
          {names.map((name) => {
            const mark = want.get(name) ?? 'none';
            const n = on.get(name) ?? 0;
            return (
              <label
                key={name}
                data-testid="tag-box"
                className={`relative inline-flex max-w-full cursor-pointer items-center gap-1.5 rounded-full border py-1.25 ps-2.25 pe-2.75 text-small has-focus-visible:[outline:var(--ring)] has-focus-visible:outline-offset-1 ${CHIP[mark]}`}
              >
                <input
                  type="checkbox"
                  className="pointer-events-none absolute m-0 size-px opacity-0"
                  checked={mark === 'all'}
                  ref={(box) => {
                    if (box) box.indeterminate = mark === 'some';
                  }}
                  onChange={() => {
                    setMark(name, nextMark(mark, start.get(name) ?? 'none'));
                  }}
                />
                <span
                  data-testid="mark"
                  aria-hidden="true"
                  className="w-3.5 flex-none text-center font-mono text-small leading-none font-semibold"
                >
                  {MARK_WORD[mark]}
                </span>
                <span data-testid="name" className="min-w-0 truncate">
                  {name}
                </span>
                {n > 0 && opened && n < opened.n ? (
                  <span data-testid="n" className="font-mono text-label leading-none tabular-nums">
                    {`${String(n)}/${String(opened.n)}`}
                  </span>
                ) : null}
              </label>
            );
          })}
          <Button id="tag-new" wraps hidden={!fresh(want, find)} onClick={make}>
            {t('ui_tag_create', find.trim())}
          </Button>
        </fieldset>
        <DialogActions>
          <DialogClose words="ui_cancel" />
          <Button id="tag-apply" kind="primary" type="submit" disabled={!changed(start, want)}>
            {t('ui_tag_apply')}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
