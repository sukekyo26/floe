import { useLayoutEffect, useRef, useState } from 'react';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { FIELD, HINT, TEXT } from '@/components/fields';
import { ChevronIcon, EditIcon } from '@/components/icons';
import { saveEdit as pageSave, type EditSaved, type EditValues } from '@/features/page-edit/stores/save-edit';
import { useHold } from '@/hooks/use-hold';
import { t } from '@/lib/t';
import type { Page } from '@/types/page';
import { pageHref } from '@/utils/page-urls';

// What the edit modal is opened for: a page, and the way back to its menu by the slug it has as it closes
export interface EditTarget {
  page: Page;
  back: (slug: string) => boolean;
}

interface EditDialogProps {
  // target is the page edited, null while closed
  readonly target: EditTarget | null;
  readonly onClosed: () => void;
  readonly save?: (
    a: Page,
    values: EditValues,
    whileHeld: (write: () => Promise<void>) => Promise<boolean>,
  ) => Promise<EditSaved>;
}

const valuesOf = (a: Page | null): EditValues => ({
  title: a?.title ?? '',
  slug: a?.slug ?? '',
  project: a?.project ?? '',
  description: a?.description ?? '',
});

// editNow is the folded row's words of what the slug and the project are, as typed
export const editNow = (values: EditValues): string =>
  `${values.slug.trim()} · ${values.project.trim() || t('ui_no_project')}`;

// EditDialog changes a page's title, slug, project and description: the slug and the project folded, with what they are
// now. A refusal is said in the modal, which stays open, the focus on the field sent that it may be; renamed whole, the
// page goes to its new name and the modal goes with it
export function EditDialog({ target, onClosed, save = pageSave }: EditDialogProps) {
  const hold = useHold();
  const [seen, setSeen] = useState<EditTarget | null>(null);
  const [values, setValues] = useState(valuesOf(null));
  const [error, setError] = useState('');
  const [more, setMore] = useState(false);
  // The way back and the slug the page has as it closes (renamed, its row's menu is under the new one)
  const [back, setBack] = useState<EditTarget['back'] | null>(null);
  // Set with onClosed() in one task, so drawn in the one render that closes it (React batches them): afterClose reads
  // the slug as renamed
  const [slug, setSlug] = useState('');
  // A refusal each: the effect below runs for every one, the same words again too
  const [refusals, setRefusals] = useState(0);
  const slugField = useRef<HTMLInputElement>(null);
  const projectField = useRef<HTMLInputElement>(null);
  const fold = useRef<HTMLDetailsElement>(null);
  // The field a refusal sends the focus to, once the hold has let go (a disabled field takes no focus); this runs
  // after the Dialog's, which gives the focus back to what had it
  const refused = useRef<HTMLInputElement | null>(null);
  useLayoutEffect(() => {
    if (!refused.current || hold.held) return;
    if (fold.current) fold.current.open = true;
    refused.current.focus();
    refused.current = null;
  }, [refusals, hold.held]);
  if (target !== seen) {
    setSeen(target);
    if (target) {
      setValues(valuesOf(target.page));
      setError('');
      setMore(false);
      setBack(() => target.back);
      setSlug(target.page.slug);
    }
  }
  const a = target?.page ?? null;
  const field = (key: keyof EditValues) => ({
    value: values[key],
    autoComplete: 'off',
    onChange: (e: { target: { value: string } }) => {
      setValues({ ...values, [key]: e.target.value });
    },
  });

  return (
    <Dialog
      id="edit-dialog"
      open={target !== null}
      onClosed={onClosed}
      afterClose={() => back?.(slug) ?? false}
      hold={hold}
      labelledBy="edit-title"
    >
      <form
        id="edit"
        className="grid gap-4"
        // A field refused inside the closed fold opens it, before the browser says why
        onInvalidCapture={(e) => {
          if (e.target instanceof HTMLElement && e.target.closest('details')) setMore(true);
        }}
        onSubmit={(e) => {
          e.preventDefault();
          if (!a) return;
          void save(a, values, hold.whileHeld).then((saved) => {
            if (!saved.ran || saved.away) return;
            setSlug(saved.slug);
            if (!saved.reason) {
              onClosed();
              return;
            }
            setError(t('ui_save_failed', saved.reason));
            if (!saved.field) return;
            refused.current = (saved.field === 'slug' ? slugField : projectField).current;
            setMore(true);
            setRefusals((n) => n + 1);
          });
        }}
      >
        <DialogHead
          icon={<EditIcon />}
          titleId="edit-title"
          title={t('ui_edit')}
          lead={<span id="edit-target">{a ? (a.path ?? pageHref({ slug: a.slug })) : ''}</span>}
        />
        <label data-testid="field" className={FIELD}>
          {t('ui_title')}
          <input
            id="edit-name"
            data-testid="text"
            className={TEXT}
            placeholder={t(a?.path ? 'ui_title_from_file' : 'ui_title_from_html')}
            {...field('title')}
          />
        </label>
        <details
          id="edit-more"
          ref={fold}
          open={more}
          onToggle={(e) => {
            setMore(e.currentTarget.open);
          }}
          className="group/more rounded-small border border-line px-3"
        >
          <summary className="-mx-3 flex cursor-pointer list-none items-center gap-1.5 px-3 py-2.25 text-small font-semibold [&::-webkit-details-marker]:hidden [&>svg]:size-3.5 [&>svg]:flex-none [&>svg]:transition-transform group-open/more:[&>svg]:rotate-90">
            <ChevronIcon />
            <span className="flex-none">{t('ui_edit_more')}</span>
            <small id="edit-more-now" className="min-w-0 truncate font-mono font-normal text-faint">
              {editNow(values)}
            </small>
          </summary>
          <label data-testid="field" className={`${FIELD} mt-1.5`}>
            {t('ui_slug')}
            <input
              id="edit-slug"
              ref={slugField}
              data-testid="path"
              className={`${TEXT} font-mono`}
              required
              spellCheck={false}
              pattern="[a-z0-9]([a-z0-9\-]{0,62}[a-z0-9])?"
              title={t('ui_slug_rule')}
              {...field('slug')}
            />
            <small className={HINT}>{t('ui_edit_slug_hint')}</small>
          </label>
          <label data-testid="field" className={`${FIELD} mt-3.5 mb-3.5`}>
            {t('ui_project')}
            <input
              id="edit-project"
              ref={projectField}
              data-testid="text"
              className={TEXT}
              spellCheck={false}
              {...field('project')}
            />
            <small className={HINT}>{t('ui_project_hint')}</small>
          </label>
        </details>
        <label data-testid="field" className={FIELD}>
          <span>
            {t('ui_description')} <small className={HINT}>{t('ui_optional')}</small>
          </span>
          <input id="edit-desc" data-testid="text" className={TEXT} {...field('description')} />
        </label>
        <p
          id="edit-error"
          role="alert"
          hidden={!error}
          className="m-0 rounded-menu bg-danger-soft px-3.5 py-2.5 whitespace-pre-wrap text-danger"
        >
          {error}
        </p>
        <DialogActions>
          <DialogClose words="ui_cancel" />
          <Button id="edit-submit" kind="primary" type="submit">
            {t('ui_save')}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
