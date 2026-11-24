import { useEffect } from 'react';

import { FrameRoute } from '@/app/routes/frame';
import { ListRoute } from '@/app/routes/list';
import { TasksRoute } from '@/app/routes/tasks';
import { DrawGuard } from '@/components/draw-guard';
import { ErrorBar } from '@/components/error-bar';
import { backToRow } from '@/components/focus-places';
import { Toasts } from '@/components/toasts';
import { getBoot, screenOf } from '@/config/boot';
import { order } from '@/stores/order';
import { pages } from '@/stores/pages';

// cameBack tells a page come back to by the browser's own way back
const cameBack = (): boolean => {
  const entry = performance.getEntriesByType('navigation')[0];
  return entry instanceof PerformanceNavigationTiming && entry.type === 'back_forward';
};

// App is the screen the server chose (boot's screen: the page list, the task list or, ?task=, a task's page, or a page
// whole), in a column of the window. The reads and their watch, and the way back, are the app's own, outside the
// DrawGuard, so that a draw that throws is drawn again by the next read; the mistake's words and the toasts stand outside
// it too
export function App({ search }: { readonly search: string }) {
  const screen = screenOf(getBoot(), search);
  useEffect(() => {
    // Back on a list, what the page whole kept for j and k is over; a frame keeps it
    if (screen !== 'frame') order.comeBack(cameBack());
    const shown = (e: PageTransitionEvent) => {
      if (!e.persisted || screen === 'frame') return;
      order.comeBack(true);
      backToRow();
    };
    addEventListener('pageshow', shown);
    void pages.load();
    const stop = pages.watch();
    return () => {
      removeEventListener('pageshow', shown);
      stop();
    };
  }, [screen]);
  return (
    <div className="flex h-dvh flex-col">
      <DrawGuard>
        {screen === 'list' ? <ListRoute /> : null}
        {screen === 'taskList' || screen === 'taskPage' ? (
          <TasksRoute name={screen === 'taskPage' ? new URLSearchParams(search).get('task') : null} />
        ) : null}
        {screen === 'frame' ? <FrameRoute /> : null}
      </DrawGuard>
      <ErrorBar />
      {/* A page whole prints the page's URL alone */}
      <div className={screen === 'frame' ? 'contents print:hidden' : 'contents'}>
        <Toasts />
      </div>
    </div>
  );
}
