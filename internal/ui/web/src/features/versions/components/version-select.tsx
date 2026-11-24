import { useEffect, useEffectEvent, useRef } from 'react';

import { PickSelect } from '@/components/pick-select';
import { versions as sharedVersions, type Versions } from '@/features/versions/stores/versions';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import type { OpenAt } from '@/stores/open-page';
import type { Page } from '@/types/page';

interface VersionSelectProps {
  // at is the page shown (picked beside the list, frameAt whole), page its entry in the list of pages. A null page is
  // one the list said it does not hold: the choice is not placed before the list was read once, as wanting the versions
  // of no page ends the words of a version gone the frame starts with
  readonly at: OpenAt | null;
  readonly page: Page | null;
  // framed: the frame's choice (frame-version), else the one over the page beside the list (read-version)
  readonly framed?: boolean;
  readonly versions?: Versions;
}

// VersionSelect is the choice of the versions of the page shown: newest first, the latest's words told, and closed the
// number alone. It is always there, hidden while nothing is to be chosen: drawn anew, an open choice would close
export function VersionSelect({ at, page, framed = false, versions = sharedVersions }: VersionSelectProps) {
  const state = useStore(versions.state);
  const select = useRef<HTMLSelectElement>(null);
  const view = versions.view(at, page).select;
  // Wanted as what is read of them changes, not at each new object of the same page (the frame makes one each draw)
  const wanted = [
    at?.slug,
    at?.file,
    at?.v,
    page?.slug,
    page?.path,
    page?.version,
    page?.versionCount,
    page?.news,
  ].join('\n');
  const want = useEffectEvent(() => {
    versions.want(at, page);
  });
  useEffect(() => {
    want();
  }, [wanted]);
  const shown = view !== null && !view.hidden;
  useEffect(() => {
    if (versions.takeFocus(view)) select.current?.focus();
  }, [versions, state.focus, view]);
  return (
    <PickSelect
      ref={select}
      id={framed ? 'frame-version' : 'read-version'}
      label={t('ui_version')}
      value={view ? String(view.value) : ''}
      onChange={(e) => {
        versions.pick(Number(e.currentTarget.value), {
          focus: false,
          held: e.currentTarget.contains(document.activeElement),
        });
      }}
      compact
      tone={view?.old ? 'old' : undefined}
      hidden={!shown}
      numbers={view?.numbers}
    >
      {/* The select's own button, named by the select's label: it shows the chosen option's copy */}
      {/* oxlint-disable-next-line jsx-a11y/control-has-associated-label -- the select's part, labelled by it */}
      <button type="button">
        <selectedcontent />
      </button>
      {view?.options.map((o) => (
        <option key={o.n} value={o.n}>
          <span>{`v${String(o.n)}`}</span>
          {/* Closed, the number alone */}
          <span className="in-[selectedcontent]:hidden">{` · ${o.rest}`}</span>
        </option>
      ))}
    </PickSelect>
  );
}
