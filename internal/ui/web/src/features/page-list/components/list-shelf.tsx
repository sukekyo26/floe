import type { ReactNode } from 'react';

import { getBoot } from '@/config/boot';
import { ArchiveIcon, FolderMark, LinkMark, PageMark, StarMark } from '@/components/icons';
import { Shelf } from '@/components/shelf';
import { ShelfAdd, ShelfAddButton } from '@/components/shelf-add';
import { Chip } from '@/components/shelf-chip';
import { ChipGroup } from '@/components/shelf-chips';
import { SHELF_MARK } from '@/components/shelf-chip';
import { ViewButton } from '@/components/shelf-views';
import { showResults } from '@/features/page-list/stores/show-results';
import {
  chipPress,
  listEmpty,
  narrowWords,
  viewCounts,
  viewLit,
  viewPress,
  viewWords,
  type Picks,
} from '@/features/page-list/utils/list-shelf';
import { useStore } from '@/hooks/use-store';
import { projectWord, t } from '@/lib/t';
import { columns as pageColumns, type ColumnsStore } from '@/stores/columns';
import { listQuery, type ListQuery } from '@/stores/list-query';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { arrange, inView, kindOf, NONE } from '@/utils/list-view';
import { projectColor } from '@/utils/project-color';
import { projectOrder } from '@/utils/project-order';
import { tally } from '@/utils/shelf-counts';

const MARK = 'mx-[calc((var(--spacing-shelf-mark)-8px)/2)] size-2 flex-none rounded-full wide:hidden';
const KINDS = [
  { kind: 'page', words: 'ui_kind_page', mark: <PageMark /> },
  { kind: 'link', words: 'ui_kind_link', mark: <LinkMark /> },
  { kind: 'folder', words: 'ui_kind_folder', mark: <FolderMark /> },
] as const;

export interface ListShelfProps {
  // onAdd opens the modal that adds a link (P5d's)
  readonly onAdd: () => void;
  readonly foot?: ReactNode;
  readonly query?: ListQuery;
  readonly pages?: Pages;
  readonly columns?: ColumnsStore;
}

// ListShelf is the page list's shelf: its four views and its parts, each counted as the rest of the shelf and the
// search leave
export function ListShelf({
  onAdd,
  foot,
  query = listQuery,
  pages = pagePages,
  columns = pageColumns,
}: ListShelfProps) {
  const state = useStore(query.query);
  const items = useStore(pages.state).entries;
  const { seen } = state;
  const pick = (change: Partial<Picks> | null) => {
    if (change === null) return;
    query.filter(change);
    showResults(columns);
  };
  const list = items.filter((a) => inView(a, state, seen));
  // Each part counted against the rest of the shelf, the view and the search, that part let go
  const left = (part: keyof Picks) => arrange(items, { ...state, [part]: '' }, seen);
  const counts = viewCounts(items, state, seen);
  const words = narrowWords(state);

  const ranks = projectOrder(items);
  const projects = tally(list, (a) => [a.project ?? NONE], state.project, left('project'));
  const names = [...ranks.keys()].filter((name) => projects.rows.has(name));
  // A project dropped from every entry but still picked comes after those in the order
  if (state.project && state.project !== NONE && !names.includes(state.project)) names.push(state.project);
  if (projects.rows.has(NONE)) names.push(NONE);
  const kinds = tally(list, (a) => [kindOf(a)], state.kind, left('kind'));
  const shownKinds = KINDS.filter(({ kind }) => kinds.rows.has(kind));
  const templates = tally(list, (a) => (a.template ? [a.template] : []), state.template, left('template'));
  const tags = tally(list, (a) => a.tags ?? [], state.tag, left('tag'));
  const byName = (x: string, y: string) => x.localeCompare(y, getBoot().lang);
  const addHidden = state.view === 'archived' || listEmpty(items, state, seen);

  const views = (
    <>
      <ViewButton
        view="new"
        mark={<span data-testid="dot" className={`${MARK} bg-new`} />}
        label={viewWords('new')}
        n={counts.new}
        lit={viewLit(state, 'new')}
        onPress={() => {
          pick(viewPress(state, 'new'));
        }}
      />
      <ViewButton
        view="all"
        mark={<span data-testid="ring" className={`${MARK} border-[1.5px] border-box-line`} />}
        label={viewWords('all')}
        n={counts.all}
        lit={viewLit(state, 'all')}
        onPress={() => {
          pick(viewPress(state, 'all'));
        }}
      />
      <ViewButton
        view="fav"
        mark={<StarMark />}
        markColor="[&_svg]:text-star"
        label={viewWords('fav')}
        labelFrom="wide"
        n={counts.fav}
        lit={viewLit(state, 'fav')}
        onPress={() => {
          pick(viewPress(state, 'fav'));
        }}
      />
      <ViewButton
        view="archived"
        mark={<ArchiveIcon />}
        markColor={`${SHELF_MARK} wide:aria-pressed:[&_svg]:text-on-ink`}
        label={viewWords('archived')}
        labelFrom="narrow"
        countFolds
        n={counts.archived}
        lit={viewLit(state, 'archived')}
        onPress={() => {
          pick(viewPress(state, 'archived'));
        }}
      />
    </>
  );
  const more = (
    <>
      <ChipGroup
        id="shelf-projects"
        boxId="chips"
        title={t('ui_project')}
        hidden={names.length === 0 && !state.project}
      >
        {names.map((name) => (
          <Chip
            key={name}
            label={projectWord(name)}
            n={projects.n(name)}
            pressed={name === state.project}
            dot={projectColor(name, ranks)}
            data={{ 'data-project': name }}
            onPick={() => {
              pick(chipPress(state, 'project', name));
            }}
          />
        ))}
      </ChipGroup>
      <ChipGroup id="shelf-kinds" boxId="kinds" title={t('ui_kind')} hidden={shownKinds.length < 2 && !state.kind}>
        {shownKinds.map(({ kind, words: key, mark }) => (
          <Chip
            key={kind}
            label={t(key)}
            n={kinds.n(kind)}
            pressed={kind === state.kind}
            mark={mark}
            data={{ 'data-kind': kind }}
            onPick={() => {
              pick(chipPress(state, 'kind', kind));
            }}
          />
        ))}
      </ChipGroup>
      {/* Under what it adds: the kinds. Shown even with the kinds hidden */}
      <ShelfAdd hidden={addHidden}>
        <ShelfAddButton
          id="add-link"
          short={t('ui_add_link_short')}
          label={t('ui_add_link')}
          hidden={addHidden}
          onAdd={onAdd}
        />
      </ShelfAdd>
      <ChipGroup id="shelf-templates" boxId="templates" title={t('ui_template')} hidden={templates.rows.size === 0}>
        {[...templates.rows.keys()]
          .sort((x, y) => (templates.rows.get(y) ?? 0) - (templates.rows.get(x) ?? 0) || byName(x, y))
          .map((name) => (
            <Chip
              key={name}
              label={name}
              n={templates.n(name)}
              pressed={name === state.template}
              data={{ 'data-template': name }}
              onPick={() => {
                pick(chipPress(state, 'template', name));
              }}
            />
          ))}
      </ChipGroup>
      <ChipGroup id="shelf-tags" boxId="tags" title={t('ui_tags')} hidden={tags.rows.size === 0}>
        {[...tags.rows.keys()].sort(byName).map((name) => (
          <Chip
            key={name}
            label={name}
            n={tags.n(name)}
            pressed={name === state.tag}
            data={{ 'data-tag': name }}
            onPick={() => {
              pick(chipPress(state, 'tag', name));
            }}
          />
        ))}
      </ChipGroup>
    </>
  );
  return (
    <Shelf
      views={views}
      narrow={{ words: words.button, n: words.n, tint: null }}
      more={more}
      {...(foot === undefined ? {} : { foot })}
    />
  );
}
