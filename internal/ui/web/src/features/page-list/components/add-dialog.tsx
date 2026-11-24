import { useState } from 'react';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { FIELD, HINT, TEXT } from '@/components/fields';
import { LinkMark } from '@/components/icons';
import { addLink as pageAddLink, type AddLink, type AddValues } from '@/features/page-list/stores/add-link';
import { useHold } from '@/hooks/use-hold';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';

export interface AddDialogProps {
  readonly open: boolean;
  readonly onClosed: () => void;
  readonly add?: AddLink;
}

// AddDialog registers a file or a folder by its path: the server tells one from the other. A refusal is said in the
// modal, which stays open; what was typed stays as it closes unsent
export function AddDialog({ open, onClosed, add = pageAddLink }: AddDialogProps) {
  const hold = useHold();
  const values = useStore(add.values);
  const [error, setError] = useState('');
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setError('');
  }
  const field = (key: keyof AddValues) => ({
    value: values[key],
    autoComplete: 'off',
    onChange: (e: { target: { value: string } }) => {
      add.values.set({ ...add.values.get(), [key]: e.target.value });
    },
  });
  return (
    <Dialog id="add-dialog" open={open} onClosed={onClosed} hold={hold} labelledBy="add-title">
      <form
        id="add"
        className="grid gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          void add.send(hold.whileHeld, onClosed).then((sent) => {
            if (sent && !sent.added) setError(t('ui_add_failed', sent.reason));
          });
        }}
      >
        <DialogHead icon={<LinkMark />} titleId="add-title" title={t('ui_add_link')} lead={t('ui_add_lead')} />
        <label data-testid="field" className={FIELD}>
          {t('ui_add_path')}
          <input
            id="add-path"
            data-testid="path"
            className={`${TEXT} font-mono`}
            required
            // The field the modal focuses as it opens (Dialog)
            data-autofocus=""
            spellCheck={false}
            placeholder="/home/you/project/coverage/index.html"
            {...field('path')}
          />
        </label>
        <label data-testid="field" className={FIELD}>
          <span>
            {t('ui_slug')} <small className={HINT}>{t('ui_optional')}</small>
          </span>
          <input
            id="add-slug"
            data-testid="text"
            className={`${TEXT} font-mono`}
            spellCheck={false}
            pattern="[a-z0-9]([a-z0-9\-]{0,62}[a-z0-9])?"
            title={t('ui_slug_rule')}
            placeholder={t('ui_add_slug_placeholder')}
            {...field('slug')}
          />
          <small className={HINT}>{t('ui_add_slug_hint')}</small>
        </label>
        <label data-testid="field" className={FIELD}>
          <span>
            {t('ui_description')} <small className={HINT}>{t('ui_optional')}</small>
          </span>
          <input id="add-desc" data-testid="text" className={TEXT} {...field('description')} />
        </label>
        <p
          id="add-error"
          role="alert"
          hidden={!error}
          className="m-0 rounded-menu bg-danger-soft px-3.5 py-2.5 whitespace-pre-wrap text-danger"
        >
          {error}
        </p>
        <DialogActions>
          <DialogClose id="add-cancel" words="ui_cancel" />
          <Button id="add-submit" kind="primary" type="submit">
            {t('ui_add')}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
