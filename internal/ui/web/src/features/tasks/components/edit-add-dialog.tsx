import { useState } from 'react';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { FIELD, SAID, TEXT } from '@/components/fields';
import { focusOn } from '@/components/focus-places';
import { PlusIcon } from '@/components/icons';
import { t } from '@/lib/t';
import type { FocusBack } from '@/types/focus';

// What the add modal is opened for: a URL or a category, and the way back to the button that opened it
export interface EditAddOpen {
  kind: 'url' | 'category';
  back: FocusBack;
}

export interface EditAddDialogProps {
  readonly open: EditAddOpen | null;
  // name is the task's, under the title
  readonly name: string;
  // refusal is why what was typed (trimmed) is not added, '' to add it
  readonly refusal: (typed: string) => string;
  readonly onAdd: (typed: string) => void;
  readonly onClosed: () => void;
}

// EditAddDialog adds a URL or a category to the task's edit modal, over it: one field, Cancel and Add. A refusal is said
// under the field, the modal open; once added, the caller closes it. It stands beside the edit modal, not in it: in it,
// its Escape would reach the edit modal's cancel too, and its form would be nested in the edit modal's, sending both
export function EditAddDialog({ open, name, refusal, onAdd, onClosed }: EditAddDialogProps) {
  // The last opened, which the modal shows and goes back from as it closes (open is null by then), and how many times
  // it was opened: its field is drawn anew, empty, for each
  const [opened, setOpened] = useState<{ last: EditAddOpen | null; n: number }>({ last: null, n: 0 });
  if (open !== null && open !== opened.last) setOpened({ last: open, n: opened.n + 1 });
  return (
    <Dialog
      id="edit-add-dialog"
      open={open !== null}
      onClosed={onClosed}
      labelledBy="edit-add-title"
      afterClose={() => opened.last?.back() ?? false}
    >
      <EditAddBody key={opened.n} url={opened.last?.kind !== 'category'} name={name} refusal={refusal} onAdd={onAdd} />
    </Dialog>
  );
}

function EditAddBody({
  url,
  name,
  refusal,
  onAdd,
}: Pick<EditAddDialogProps, 'name' | 'refusal' | 'onAdd'> & { readonly url: boolean }) {
  const [typed, setTyped] = useState('');
  const [said, setSaid] = useState('');
  return (
    <form
      className="grid gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        const x = typed.trim();
        const why = refusal(x);
        setSaid(why);
        if (why) focusOn(document.getElementById('edit-add-new'));
        else onAdd(x);
      }}
    >
      <DialogHead
        icon={<PlusIcon />}
        titleId="edit-add-title"
        title={t(url ? 'ui_url_add' : 'ui_category_add')}
        lead={name}
        leadId="edit-add-target"
      />
      <label data-testid="field" className={FIELD}>
        {t(url ? 'ui_task_url' : 'ui_category_name')}
        <input
          id="edit-add-new"
          data-testid={url ? 'path' : 'text'}
          className={url ? `${TEXT} font-mono` : TEXT}
          autoComplete="off"
          spellCheck={url ? false : undefined}
          placeholder={url ? 'https://' : undefined}
          aria-describedby="edit-add-said"
          data-autofocus=""
          value={typed}
          onChange={(e) => {
            setTyped(e.currentTarget.value);
          }}
          // On every input, as onChange skips the same words typed again
          onInput={() => {
            setSaid('');
          }}
        />
      </label>
      <p id="edit-add-said" aria-live="polite" className={SAID}>
        {said}
      </p>
      <DialogActions>
        <DialogClose words="ui_cancel" />
        <Button id="edit-add-submit" kind="primary" type="submit">
          {t('ui_add')}
        </Button>
      </DialogActions>
    </form>
  );
}
