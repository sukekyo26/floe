import { useLayoutEffect, useRef, useState, type ReactNode } from 'react';

import type { Screen } from '@/types/screen';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { HelpIcon } from '@/components/icons';
import { Tab, TabList } from '@/components/tabs';
import { t } from '@/lib/t';

export type HelpTab = 'about' | 'keys';

interface KeysDialogProps {
  // open is the tab it opens on (? the keys, the bar's button the about), null while closed
  readonly open: HelpTab | null;
  readonly screen: Screen;
  // diff tells the diff of a page whole, whose keys d, n and p it tells
  readonly diff: boolean;
  // diffGo tells that d goes to a diff where it opens, as the bar's diff button is shown
  readonly diffGo: boolean;
  // backWords are h's on a page whole: the back button's whole words
  readonly backWords: string;
  readonly mac: boolean;
  readonly onClosed: () => void;
}

const KBD =
  'rounded-small border border-line bg-subtle px-1.5 py-0.75 font-mono text-label leading-none whitespace-nowrap text-muted';
const DL = 'm-0 grid items-baseline gap-x-3 gap-y-1.5 text-small';
const KEYS_DL = `${DL} grid-cols-[11rem_minmax(0,1fr)] [&_dd]:m-0 [&_dt]:flex [&_dt]:items-center [&_dt]:gap-1`;
const H3 = 'mt-0 mb-1.25 text-label font-[650] text-muted';
const NOTE = 'm-0 text-small text-muted';

const Kbd = ({ children, mod = false }: { readonly children: ReactNode; readonly mod?: boolean }) =>
  mod ? (
    <kbd data-testid="mod" className={KBD}>
      {children}
    </kbd>
  ) : (
    <kbd className={KBD}>{children}</kbd>
  );
const Or = () => <i className="text-label whitespace-nowrap text-faint not-italic">{t('ui_keys_or')}</i>;

// KeysDialog is the help: what the screen's things are (about) and its keys. Its parts are all there, open or closed,
// those of another screen hidden (the tests read them unopened)
export function KeysDialog({ open, screen, diff, diffGo, backWords, mac, onClosed }: KeysDialogProps) {
  const [tab, setTab] = useState<HelpTab>('keys');
  const [seen, setSeen] = useState<HelpTab | null>(null);
  const panels = useRef<HTMLDivElement>(null);
  if (open !== seen) {
    setSeen(open);
    if (open) setTab(open);
  }
  // Each tab from its top: a closed modal has no layout, so this runs once it is open
  useLayoutEffect(() => {
    if (open && panels.current) panels.current.scrollTop = 0;
  }, [open, tab]);
  const tasks = screen === 'taskList' || screen === 'taskPage';
  const frame = screen === 'frame';
  const mod = mac ? '⌘' : 'Ctrl';
  const note = (key: 'ui_keys_note_list' | 'ui_keys_note_frame') => (mac ? t(key).replace('Ctrl+K', '⌘K') : t(key));
  return (
    <Dialog id="keys" open={open !== null} onClosed={onClosed} labelledBy="keys-title" body="help">
      <DialogHead icon={<HelpIcon />} titleId="keys-title" title={t('ui_help')} />
      <TabList label={t('ui_help')}>
        {(['about', 'keys'] as const).map((name) => (
          <Tab
            key={name}
            id={`help-tab-${name}`}
            help={name}
            selected={tab === name}
            onSelect={() => {
              setTab(name);
            }}
          >
            {name === 'about' ? (
              <>
                <span hidden={tasks}>{t('ui_help_about_pages')}</span>
                <span hidden={!tasks}>{t('ui_help_about_tasks')}</span>
              </>
            ) : (
              t('ui_keys')
            )}
          </Tab>
        ))}
      </TabList>
      <div ref={panels} data-testid="help-panels" className="min-h-0 overflow-y-auto">
        <div
          id="help-about"
          role="tabpanel"
          aria-labelledby="help-tab-about"
          tabIndex={tab === 'about' ? 0 : undefined}
          hidden={tab !== 'about'}
          className="grid gap-4"
        >
          <div hidden={tasks} className="grid gap-4">
            <p data-testid="help-lead" className={NOTE}>
              {t('ui_help_pages_text')}
            </p>
            <dl className={`${DL} grid-cols-[7.5rem_minmax(0,1fr)] [&_dd]:m-0 [&_dt]:font-semibold`}>
              <dt>{t('ui_view_new')}</dt>
              <dd>{t('ui_help_new_text')}</dd>
              <dt>{t('ui_view_favorites')}</dt>
              <dd>{t('ui_help_favorites_text')}</dd>
              <dt>{t('ui_state_archived')}</dt>
              <dd>{t('ui_help_archive_text')}</dd>
              <dt>{t('ui_project')}</dt>
              <dd>{t('ui_help_project_pages_text')}</dd>
            </dl>
          </div>
          <div hidden={!tasks} className="grid gap-4">
            <p data-testid="help-lead" className={NOTE}>
              {t('ui_help_tasks_text')}
            </p>
            {/* LIMIT: the four states are named here: add a state that store.States comes to have */}
            <dl className={`${DL} grid-cols-[7.5rem_minmax(0,1fr)] [&_dd]:m-0 [&_dt]:font-semibold`}>
              <dt>{t('ui_project')}</dt>
              <dd>{t('ui_help_project_tasks_text')}</dd>
              <dt>{t('ui_help_task')}</dt>
              <dd>{t('ui_help_task_text')}</dd>
              <dt>{t('ui_help_state')}</dt>
              <dd>
                {t(
                  'ui_help_state_text',
                  t('ui_state_todo'),
                  t('ui_state_doing'),
                  t('ui_state_done'),
                  t('ui_state_archived'),
                )}
              </dd>
              <dt>{t('ui_task_category')}</dt>
              <dd>{t('ui_help_category_text')}</dd>
              <dt>{t('ui_pins')}</dt>
              <dd>{t('ui_help_pin_text')}</dd>
            </dl>
          </div>
        </div>
        <div
          id="help-keys"
          role="tabpanel"
          aria-labelledby="help-tab-keys"
          tabIndex={tab === 'keys' ? 0 : undefined}
          hidden={tab !== 'keys'}
          className="grid gap-4"
        >
          <p id="keys-note-list" hidden={frame} className={NOTE}>
            {note('ui_keys_note_list')}
          </p>
          <p id="keys-note-frame" hidden={!frame} className={NOTE}>
            {note('ui_keys_note_frame')}
          </p>
          <div id="keys-list" data-testid="keys-part" hidden={frame} className="grid gap-4">
            <section>
              <h3 className={H3}>{t('ui_keys_move')}</h3>
              <dl className={KEYS_DL}>
                <dt>
                  <Kbd>j</Kbd>
                  <Or />
                  <Kbd>↓</Kbd>
                </dt>
                <dd>{t('ui_keys_next_row')}</dd>
                <dt>
                  <Kbd>k</Kbd>
                  <Or />
                  <Kbd>↑</Kbd>
                </dt>
                <dd>{t('ui_keys_prev_row')}</dd>
                <dt>
                  <Kbd>Enter</Kbd>
                </dt>
                <dd id="keys-enter">{t(screen === 'taskList' ? 'ui_keys_open_task' : 'ui_keys_whole')}</dd>
                {screen === 'taskPage' ? (
                  <>
                    <dt>
                      <Kbd>h</Kbd>
                    </dt>
                    <dd>{t('ui_tasks_back')}</dd>
                  </>
                ) : null}
              </dl>
            </section>
            <section id="keys-row" hidden={screen === 'taskList'}>
              <h3 className={H3}>{t('ui_keys_row')}</h3>
              <dl className={KEYS_DL}>
                <dt data-testid="select-key" hidden={screen === 'taskPage'}>
                  <Kbd>x</Kbd>
                </dt>
                <dd data-testid="select-key" hidden={screen === 'taskPage'}>
                  {t('ui_keys_select')}
                </dd>
                <dt>
                  <Kbd>f</Kbd>
                </dt>
                <dd>{t('ui_view_favorites')}</dd>
                <dt data-testid="diff-go" hidden={!diffGo}>
                  <Kbd>d</Kbd>
                </dt>
                <dd data-testid="diff-go" hidden={!diffGo}>
                  {t('ui_keys_diff')}
                </dd>
              </dl>
            </section>
            <section>
              <h3 className={H3}>{t('ui_keys_open_part')}</h3>
              <dl className={KEYS_DL}>
                <dt>
                  <Kbd>/</Kbd>
                </dt>
                <dd>{t('ui_keys_search')}</dd>
                <dt>
                  <Kbd mod>{mod}</Kbd>
                  <Kbd>K</Kbd>
                </dt>
                <dd>{t('ui_keys_find')}</dd>
                <dt>
                  <Kbd>?</Kbd>
                </dt>
                <dd>{t('ui_keys_this')}</dd>
                <dt>
                  <Kbd>Esc</Kbd>
                </dt>
                <dd>{t('ui_keys_close')}</dd>
              </dl>
            </section>
          </div>
          <div id="keys-frame" data-testid="keys-part" hidden={!frame} className="grid gap-4">
            <section>
              <h3 className={H3}>{t('ui_keys_move')}</h3>
              <dl className={KEYS_DL}>
                <dt>
                  <Kbd>j</Kbd>
                </dt>
                <dd>{t('ui_keys_next_page')}</dd>
                <dt>
                  <Kbd>k</Kbd>
                </dt>
                <dd>{t('ui_keys_prev_page')}</dd>
                <dt>
                  <Kbd>h</Kbd>
                </dt>
                <dd id="keys-back">{backWords || t('ui_to_page_list')}</dd>
              </dl>
            </section>
            <section>
              <h3 className={H3}>{t('ui_keys_page')}</h3>
              <dl className={KEYS_DL}>
                <dt>
                  <Kbd>f</Kbd>
                </dt>
                <dd>{t('ui_view_favorites')}</dd>
                <dt data-testid="diff-go" hidden={!diffGo}>
                  <Kbd>d</Kbd>
                </dt>
                <dd data-testid="diff-go" hidden={!diffGo}>
                  {t('ui_keys_diff')}
                </dd>
                <dt data-testid="diff-key" hidden={!diff}>
                  <Kbd>d</Kbd>
                </dt>
                <dd data-testid="diff-key" hidden={!diff}>
                  {t('ui_diff_close')}
                </dd>
                <dt data-testid="diff-key" hidden={!diff}>
                  <Kbd>n</Kbd>
                </dt>
                <dd data-testid="diff-key" hidden={!diff}>
                  {t('ui_diff_next')}
                </dd>
                <dt data-testid="diff-key" hidden={!diff}>
                  <Kbd>p</Kbd>
                </dt>
                <dd data-testid="diff-key" hidden={!diff}>
                  {t('ui_diff_prev')}
                </dd>
              </dl>
            </section>
            <section>
              <h3 className={H3}>{t('ui_keys_open_part')}</h3>
              <dl className={KEYS_DL}>
                <dt>
                  <Kbd>/</Kbd>
                  <Or />
                  <Kbd mod>{mod}</Kbd>
                  <Kbd>K</Kbd>
                </dt>
                <dd>{t('ui_keys_find')}</dd>
                <dt>
                  <Kbd>?</Kbd>
                </dt>
                <dd>{t('ui_keys_this')}</dd>
                <dt>
                  <Kbd>Esc</Kbd>
                </dt>
                <dd>{t('ui_keys_close')}</dd>
              </dl>
            </section>
          </div>
        </div>
      </div>
      <DialogActions>
        <DialogClose focusOnOpen />
      </DialogActions>
    </Dialog>
  );
}
