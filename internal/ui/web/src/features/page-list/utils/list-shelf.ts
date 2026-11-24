import type { Boot } from '@/config/boot';
import { t } from '@/lib/t';
import type { Page } from '@/types/page';
import type { MessageKey } from '@/types/messages.gen';
import { arrange, inView, unseen, type ListState } from '@/utils/list-view';

// What the shelf picks, of the page list: the view and the four parts
export type Picks = Omit<ListState, 'q'>;
export type Part = 'kind' | 'template' | 'project' | 'tag';

export const VIEWS = ['new', 'all', 'fav', 'archived'] as const;
export type View = (typeof VIEWS)[number];

export const NO_PICKS = { kind: '', template: '', project: '', tag: '' } as const;

const VIEW_WORDS: Record<View, MessageKey> = {
  new: 'ui_view_new',
  all: 'ui_view_all',
  fav: 'ui_view_favorites',
  archived: 'ui_state_archived',
};
export const viewWords = (view: View): string => t(VIEW_WORDS[view]);

// picksOn is how many of the shelf's parts narrow the list
export const picksOn = (state: ListState): number =>
  [state.kind, state.template, state.project, state.tag].filter(Boolean).length;

// viewCounts is each view's count with the picks and the search on it. All is what pressing it leaves, the search
// alone; the new ones are those new now, not those read that stay on the view
export function viewCounts(items: readonly Page[], state: ListState, seen: ReadonlySet<string>): Record<View, number> {
  return {
    all: arrange(items, { ...state, ...NO_PICKS, view: 'all' }, seen).length,
    new: arrange(items, { ...state, view: 'new' }, seen).filter((a) => a.news).length,
    fav: arrange(items, { ...state, view: 'fav' }, seen).length,
    archived: arrange(items, { ...state, view: 'archived' }, seen).length,
  };
}

// viewLit tells whether a view's button is lit: All is no view and no picks, whatever the search, which the bar shows
export const viewLit = (state: ListState, view: View): boolean =>
  view === state.view && (view !== 'all' || picksOn(state) === 0);

// viewPress is what a press of a view's button changes: lit, a view lets go of itself and All does nothing (null);
// unlit, All lets go of the view and the picks
export function viewPress(state: ListState, view: View): Partial<Picks> | null {
  if (view !== 'all') return { view: view === state.view ? 'all' : view };
  return viewLit(state, 'all') ? null : { view: 'all', ...NO_PICKS };
}

// tagPress is what a row's tag narrows the list to: the tag, or nothing for the one picked already, which leaves the
// list as it is (its selection too)
export const tagPress = (state: ListState, name: string): Partial<Picks> | null =>
  state.tag === name ? null : { tag: name };

// chipPress is what a press of a chip changes: what is picked, picked again, is let go
export const chipPress = (state: ListState, part: Part, value: string): Partial<Picks> => ({
  [part]: state[part] === value ? '' : value,
});

// newsPress is what the bar's new pages leave: every new one, as the bar counts all there is, so that nothing narrows
// them, the search neither
export const newsPress = (): { picks: Picks; q: string } => ({ picks: { view: 'new', ...NO_PICKS }, q: '' });

// newsTaken tells whether the bar's new pages narrow the list in place on a screen: the page list's alone; elsewhere
// they are a link to it
export const newsTaken = (screen: Boot['screen']): boolean => screen === 'list';

// newCount is the bar's count: every unread page not archived, whatever narrows the list
export const newCount = (items: readonly Page[]): number => items.filter(unseen).length;

// unseenSlugs is every unread page not archived, whatever narrows the list: the view "new" keeps them all as it draws
export const unseenSlugs = (items: readonly Page[]): string[] => items.filter(unseen).map((a) => a.slug);

// narrowWords is what the button to the filters and the head say of the picks: the button how many picks, the head the
// view and how many picks, with the filters folded
export function narrowWords(state: ListState): { n: number; button: string; what: string } {
  const n = picksOn(state);
  const many = n ? t('ui_narrow_n', n) : '';
  const view = state.view === 'all' ? '' : viewWords(state.view as View);
  return { n, button: many || t('ui_narrow'), what: [view, many].filter(Boolean).join(' · ') };
}

// listEmpty tells that there is no page at all to list, archived ones neither: the list says how to add one, and the
// shelf's add button gives way to it
export const listEmpty = (items: readonly Page[], state: ListState, seen: ReadonlySet<string>): boolean =>
  state.view === 'all' && !items.some((a) => a.archived) && !items.some((a) => inView(a, state, seen));
