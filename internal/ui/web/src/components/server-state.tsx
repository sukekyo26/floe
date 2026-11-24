import { useStore } from '@/hooks/use-store';
import { client as pageClient, type Client } from '@/lib/api-client';
import { t } from '@/lib/t';

// ServerState tells the state of the server, not an action: nothing shows while connected, and the words only when the
// connection is lost (a dot alone on a narrow screen, the words its name). Connected, it stays in the page, empty, as
// a status region must to be read as it changes. host is the address in the address bar, so it is not shown
// LIMIT: lost, the words in the bar's flow push the list's search, centered by its auto margins, to the right, and back
// when it is found again. Take them out of the flow if that comes to trouble someone
export function ServerState({
  client = pageClient,
  host = () => window.location.host,
}: {
  readonly client?: Client;
  readonly host?: () => string;
}) {
  const up = useStore(client.reached);
  const label = t(up ? 'ui_server_up' : 'ui_server_down_host', host());
  return (
    <output
      id="server"
      title={label}
      aria-label={label}
      data-down={up ? undefined : 'true'}
      className="sr-only data-down:not-sr-only data-down:flex data-down:items-center data-down:gap-2 data-down:rounded-full data-down:border data-down:border-down-line data-down:bg-panel data-down:px-2.75 data-down:py-1.25 data-down:text-label data-down:leading-none data-down:font-semibold data-down:whitespace-nowrap data-down:text-danger data-down:tabular-nums data-down:before:size-1.5 data-down:before:flex-none data-down:before:rounded-full data-down:before:bg-danger data-down:before:content-[''] narrow:data-down:gap-0 narrow:data-down:p-1.75 narrow:data-down:text-[0px]"
    >
      {up ? '' : t('ui_server_down')}
    </output>
  );
}
