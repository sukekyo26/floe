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
  // search is the list's search, in the bar's middle (ui.html .bar-inner > .search); none on the other screens
  readonly search?: ReactNode;
  readonly onFind: () => void;
  readonly onHelp: () => void;
  readonly onSettings: () => void;
}

// Bar is the screen's bar (ui.html header.bar): the logo, the server's state and the way between the screens, then the
// new pages and the ways to the search (none beside the search field, ui.html 6788), the help and the settings. The
// help's and the settings' are gone at 40rem or less (ui.html 708)
export function Bar({ screen, listUp, taskUp, search, onFind, onHelp, onSettings }: BarProps) {
  return (
    <header data-testid="bar" className="sticky top-0 z-10 border-b border-line bg-bg">
      <div data-testid="bar-inner" className="flex h-bar items-center gap-3 px-4">
        <Brand />
        <ServerState />
        <ScreenSwitch screen={screen} listUp={listUp} taskUp={taskUp} />
        {search}
        <div data-testid="bar-end" className="ml-auto flex flex-none items-center gap-2.5">
          <News />
          {search === undefined && (
            <IconButton id="find-open" label={t('ui_find')} onClick={onFind}>
              <SearchIcon />
            </IconButton>
          )}
          <IconButton id="keys-open" label={t('ui_help')} display="wide" onClick={onHelp}>
            <HelpIcon />
          </IconButton>
          <IconButton id="settings-open" label={t('ui_settings')} display="wide" onClick={onSettings}>
            <SettingsIcon />
          </IconButton>
        </div>
      </div>
    </header>
  );
}
