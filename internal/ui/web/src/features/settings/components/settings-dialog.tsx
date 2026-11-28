import { useLayoutEffect, useRef, useState } from 'react';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose } from '@/components/dialog';
import { BackChevronIcon, ChevronIcon, InfoIcon, SettingsIcon, TaskModeIcon, ThemeIcon } from '@/components/icons';
import { ThemeCard } from '@/features/settings/components/theme-card';
import { themeWrites as pageWrites, type ThemeWrites } from '@/features/settings/stores/choose-theme';
import { settings as pageSettings, type Settings } from '@/features/settings/stores/settings';
import { pageShown, settingsView, themeCards, type Now, type Panel } from '@/features/settings/utils/settings-view';
import { useHold } from '@/hooks/use-hold';
import { useMovedOverlay } from '@/hooks/use-moved-overlay';
import { useShrink } from '@/hooks/use-shrink';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import type { FocusBack } from '@/types/focus';
import type { Page } from '@/types/page';

// What the settings are opened for: the settings' list (page null, from the bar), or a page's themes (from its menu,
// the focus going back by back)
export interface SettingsTarget {
  page: Page | null;
  back: FocusBack | null;
}

interface SettingsDialogProps {
  // target is null while closed
  readonly target: SettingsTarget | null;
  readonly onClosed: () => void;
  readonly settings?: Settings;
  readonly writes?: ThemeWrites;
}

const ITEM =
  'flex min-h-18 cursor-pointer items-center gap-3.5 rounded-menu border border-line bg-frame-bg px-4 py-3.5 text-left text-fg shadow-sm hover:border-faint hover:bg-hover [&>svg]:size-4.5 [&>svg]:flex-none [&>svg]:text-muted';
const DOT = 'grid size-9.5 flex-none place-items-center rounded-full bg-subtle [&_svg]:size-4.5';
const WHAT = 'grid min-w-0 flex-1 gap-0.5 [&_b]:text-body [&_small]:text-small [&_small]:text-muted';

// NowLine is the line over the cards: what the theme is now, or why there are none
function NowLine({ now }: { readonly now: Now }) {
  const gone = <span className="font-semibold text-danger">{t('ui_theme_gone')}</span>;
  switch (now.kind) {
    case 'reading':
      return null;
    case 'failed':
      return now.why;
    case 'ownBroken':
      return <span className="font-semibold text-danger">{t('ui_theme_own_broken')}</span>;
    case 'settingsBroken':
      return <span className="font-semibold text-danger">{t('ui_theme_settings_broken')}</span>;
    case 'now':
      return (
        <>
          {t('ui_theme_now_label')}
          <b className="text-fg">{now.name}</b>
        </>
      );
    case 'gone':
      return (
        <>
          {t('ui_theme_now_label')}
          <b className="text-fg">{now.name}</b> {gone}
          {t('ui_theme_gone_more')}
        </>
      );
    case 'pageMore':
      return <>{t('ui_theme_page_more')}</>;
    case 'pickedGone':
      return (
        <>
          {t('ui_theme_picked_label')}
          <b className="text-fg">{now.name}</b> {gone}
          {t('ui_theme_picked_more')}
        </>
      );
  }
}

// SettingsDialog is the settings: opened on their list (the default theme, the task mode), a part gone into and back,
// or a page's themes alone. While open, the page's toasts are in it (useMovedOverlay), which say what came of a save
export function SettingsDialog({
  target,
  onClosed,
  settings = pageSettings,
  writes = pageWrites,
}: SettingsDialogProps) {
  const hold = useHold();
  const { read, own, failed } = useStore(settings.state);
  const [panel, setPanel] = useState<Panel>('list');
  const [seen, setSeen] = useState<SettingsTarget | null>(null);
  const [kept, setKept] = useState<Page | null>(null);
  const page = pageShown(kept, target);
  const [back, setBack] = useState<FocusBack | null>(null);
  // The card chosen and the task mode checked, as the person left them, drawn anew from each read
  const [choice, setChoice] = useState<{ of: unknown; name: string | null }>({ of: null, name: null });
  const [modeOn, setModeOn] = useState<{ of: unknown; on: boolean }>({ of: null, on: false });
  // Where the focus goes once a part is drawn: its heading, or the list's row of the part it came from
  const focusTo = useRef<Panel | 'title' | null>(null);
  const title = useRef<HTMLHeadingElement>(null);
  const list = useRef<HTMLDivElement>(null);
  const grid = useRef<HTMLDivElement>(null);
  const toastBox = useRef<HTMLDivElement>(null);
  if (target !== seen) {
    setSeen(target);
    setKept(page);
    if (target) {
      setPanel(target.page ? 'theme' : 'list');
      setBack(() => target.back);
    }
  }
  // Read anew each opening, before it is drawn (a layout effect: the last opening's page and cards never show), once
  // (StrictMode's second run is the same opening, asking again harmlessly)
  useLayoutEffect(() => {
    if (target) void settings.open(target.page);
  }, [target, settings]);
  const open = target !== null;
  useMovedOverlay(open, { toasts: toastBox });
  const view = settingsView(panel, page !== null, read, failed, t('ui_settings_broken'));
  const unset = own ? t('ui_theme_unset_named', own) : t('ui_theme_unset_about');
  const cards = themeCards(read, failed, page ? (page.pickedTheme ?? '') : null, unset);
  if (choice.of !== read) setChoice({ of: read, name: cards.checked });
  if (modeOn.of !== read) setModeOn({ of: read, on: Boolean(read?.taskMode) });
  useShrink(grid, read);

  useLayoutEffect(() => {
    const to = focusTo.current;
    focusTo.current = null;
    if (to === 'title') title.current?.focus();
    else if (to) list.current?.querySelector<HTMLElement>(`[data-panel="${to}"]`)?.focus();
  }, [panel]);
  // A part gone into, its heading focused; back to the list, the row of the part it came from
  const goTo = (next: Panel) => {
    focusTo.current = next === 'list' ? panel : 'title';
    // The task mode's part shows the mode as read, a check left unsaved gone
    if (next === 'mode') setModeOn({ of: read, on: Boolean(read?.taskMode) });
    setPanel(next);
  };

  return (
    <Dialog
      id="settings"
      open={open}
      onClosed={onClosed}
      afterClose={() => back?.() ?? false}
      hold={hold}
      labelledBy="settings-title"
      width={panel === 'theme' ? 'wide' : 'mid'}
      body="fill"
      panel={panel}
    >
      <form
        id="settings-form"
        className="flex min-h-0 flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (panel === 'mode') {
            void writes.saveTaskMode({
              on: modeOn.on,
              whileHeld: hold.whileHeld,
              saved: () => {
                settings.saved({ taskMode: modeOn.on });
                goTo('list');
              },
            });
            return;
          }
          if (choice.name === null) return;
          const theme = choice.name;
          void writes
            .chooseTheme({
              page,
              theme,
              whileHeld: hold.whileHeld,
              saved: () => {
                settings.saved({ default: theme });
                goTo('list');
              },
            })
            .then((done) => {
              if (page && done.ran && !done.why) onClosed();
            });
        }}
      >
        <button
          type="button"
          id="settings-back"
          aria-label={t('ui_settings_back')}
          hidden={!view.back}
          onClick={() => {
            goTo('list');
          }}
          className="inline-flex items-center gap-0.75 self-start border-0 bg-transparent p-0 text-small text-muted hover:text-fg [&>svg]:size-3.5"
        >
          <BackChevronIcon />
          {t('ui_settings')}
        </button>
        <div data-testid="modal-head" className="flex items-center gap-3.25">
          <span className="grid size-9.5 flex-none place-items-center rounded-full bg-sel p-2.25 text-fg [&_svg]:size-5">
            <ThemeIcon testid="page-icon" hidden={!page} />
            <SettingsIcon testid="settings-icon" hidden={page !== null} />
          </span>
          <div>
            <h2
              id="settings-title"
              ref={title}
              tabIndex={-1}
              className="m-0 text-[1.05rem] leading-[1.3] font-bold tracking-[-0.01em] focus:outline-none"
            >
              {t(view.title)}
            </h2>
            <p id="theme-page" data-testid="lead" hidden={!page} className="mt-0.5 mb-0 text-small text-muted">
              {page?.title ?? ''}
            </p>
          </div>
        </div>
        <p id="settings-broken" hidden={view.broken === null} className="m-0 text-small font-semibold text-danger">
          {view.broken}
        </p>
        <div id="settings-list" ref={list} hidden={!view.list} className="grid gap-2.5">
          {(['theme', 'mode'] as const).map((part) => (
            <button
              key={part}
              type="button"
              data-testid="settings-item"
              data-panel={part}
              onClick={() => {
                goTo(part);
              }}
              className={ITEM}
            >
              <span className={DOT}>{part === 'theme' ? <ThemeIcon /> : <TaskModeIcon />}</span>
              <span className={WHAT}>
                <b>
                  {t(part === 'theme' ? 'ui_settings_theme' : 'ui_task_mode')}
                  {t('ui_settings_value_sep')}
                  <span id={part === 'theme' ? 'settings-theme-now' : 'settings-mode-now'}>
                    {part === 'theme'
                      ? view.themeNow
                      : view.modeNow === '—' || view.modeNow === ''
                        ? view.modeNow
                        : t(view.modeNow)}
                  </span>
                </b>
                <small>{t(part === 'theme' ? 'ui_settings_theme_about' : 'ui_task_mode_about')}</small>
              </span>
              <ChevronIcon />
            </button>
          ))}
        </div>
        {/* The default theme is for the pages stored from now on alone: said over the cards, as its name was read as all
            the pages' */}
        <div
          id="theme-keep"
          role="note"
          hidden={!view.themes || page !== null}
          className="flex items-start gap-3 rounded-menu border border-new/40 bg-new-soft px-4 py-3 text-small text-fg [&>svg]:mt-0.5 [&>svg]:size-5 [&>svg]:flex-none [&>svg]:text-new"
        >
          <InfoIcon />
          <span className="grid gap-0.5">
            <b className="text-body">{t('ui_theme_keep')}</b>
            <span>{t('ui_theme_keep_more')}</span>
          </span>
        </div>
        <p id="theme-now" hidden={!view.themes} className="m-0 text-small text-muted">
          <NowLine now={cards.now} />
        </p>
        <div
          id="theme-grid"
          ref={grid}
          role="radiogroup"
          aria-labelledby="settings-title"
          hidden={!view.themes}
          className="-m-1 grid min-h-0 shrink grid-cols-[repeat(auto-fill,minmax(13.5rem,1fr))] gap-3.5 overflow-auto p-1 max-[600px]:grid-cols-2 max-[600px]:gap-2.5"
        >
          {cards.mistakes.length > 0 ? (
            <p className="m-0 rounded-small bg-danger-soft px-4 py-3 font-mono text-small wrap-anywhere whitespace-pre-wrap text-danger">
              {cards.mistakes.join('\n')}
            </p>
          ) : null}
          {cards.cards.map((card) => (
            <ThemeCard
              key={card.name}
              card={card}
              checked={choice.name === card.name}
              onPick={() => {
                setChoice({ of: read, name: card.name });
              }}
            />
          ))}
        </div>
        {/* oxlint-disable-next-line jsx-a11y/label-has-associated-control -- its words are the task mode's, in the spans */}
        <label
          id="settings-mode"
          hidden={!view.mode}
          className="flex cursor-pointer items-start gap-3 rounded-menu border border-line bg-frame-bg px-4 py-3.5"
        >
          <input
            type="checkbox"
            id="task-mode"
            checked={modeOn.on}
            onChange={(e) => {
              setModeOn({ of: read, on: e.target.checked });
            }}
            className="mt-0.5 size-4.5 flex-none"
          />
          <span className={WHAT}>
            <b>{t('ui_task_mode')}</b>
            <span>{t('ui_task_mode_about')}</span>
            <small>{t('ui_task_mode_more')}</small>
          </span>
        </label>
        <DialogActions>
          <DialogClose words={view.close} />
          <Button id="theme-choose" kind="primary" type="submit" hidden={!view.themes} disabled={choice.name === null}>
            {t('ui_select')}
          </Button>
          <Button id="mode-save" kind="primary" type="submit" hidden={!view.mode} disabled={!view.modeSaves}>
            {t('ui_save')}
          </Button>
        </DialogActions>
      </form>
      {/* The page's toasts, while open */}
      <div ref={toastBox} className="contents" />
    </Dialog>
  );
}
