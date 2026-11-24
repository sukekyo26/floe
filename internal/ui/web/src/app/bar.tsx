import type { ReactNode } from 'react';

import { Brand } from '@/components/brand';
import { HelpIcon, SearchIcon, SettingsIcon } from '@/components/icons';
import { IconButton } from '@/components/icon-button';
import { ScreenSwitch, type Up } from '@/components/screen-switch';
import { ServerState } from '@/components/server-state';
import { News } from '@/features/page-list/components/news';
import { t } from '@/lib/t';
import type { Screen } from '@/types/screen';

interface BarProps {
  readonly screen: Screen;
  readonly listUp?: Up | undefined;
  readonly taskUp?: Up | undefined;
  // search is the list's search, in the bar's middle: the list's, and a page whole's to look for its page not found;
  // none on the task screens
  readonly search?: ReactNode;
  // A page whole's: back, its way back to a task's page, before the header's links; lead, its name, version and marks
  // after them; actions, the page's own at the end; brand, where the logo goes; and seeking, its page not found, whose
  // bar has the list's search and new pages
  readonly back?: ReactNode;
  readonly lead?: ReactNode;
  readonly actions?: ReactNode;
  readonly brand?: string;
  readonly seeking?: boolean;
  readonly onFind: () => void;
  readonly onHelp: () => void;
  readonly onSettings: () => void;
}

// Bar is the screen's bar: the logo, the server's state and the way between the screens, then the new pages and the
// ways to the search (none on the list, beside its search field), the help and the settings. Which of them each screen
// shows is told here alone: the help's and the settings' are gone at 40rem or less, and in a page whole, which leaves
// them to the lists, whose way to the search is at 40rem or less alone. They stay in the DOM, hidden
export function Bar({
  screen,
  listUp,
  taskUp,
  search,
  back,
  lead,
  actions,
  brand,
  seeking = false,
  onFind,
  onHelp,
  onSettings,
}: BarProps) {
  const frame = screen === 'frame';
  return (
    <header data-testid="bar" className={`sticky top-0 z-10 border-b border-line bg-bg ${frame ? 'print:hidden' : ''}`}>
      <div data-testid="bar-inner" className="flex h-bar items-center gap-3 px-4">
        <Brand href={brand} markAlone={frame} />
        <ServerState />
        {back}
        <ScreenSwitch screen={screen} listUp={listUp} taskUp={taskUp} />
        {lead}
        {search}
        <div data-testid="bar-end" className="ml-auto flex flex-none items-center gap-2.5">
          {actions}
          <News hidden={frame && !seeking} />
          {screen !== 'list' && (
            <IconButton id="find-open" label={t('ui_find')} display={frame ? 'narrow' : 'shown'} onClick={onFind}>
              <SearchIcon />
            </IconButton>
          )}
          <IconButton id="keys-open" label={t('ui_help')} display={frame ? 'never' : 'wide'} onClick={onHelp}>
            <HelpIcon />
          </IconButton>
          <IconButton
            id="settings-open"
            label={t('ui_settings')}
            display={frame ? 'never' : 'wide'}
            onClick={onSettings}
          >
            <SettingsIcon />
          </IconButton>
        </div>
      </div>
    </header>
  );
}
