import { useRef, useState } from 'react';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { FIELD, SAID, TEXT } from '@/components/fields';
import { focusOn } from '@/components/focus-places';
import { PlusIcon } from '@/components/icons';
import { t } from '@/lib/t';

// What the add modal is opened for: a URL or a category, and the way back as it closes, told whether one was added
export interface AddNameOpen {
  kind: 'url' | 'category';
  back: (added: boolean) => boolean;
}

export interface AddNameDialogProps {
  readonly id: string;
  readonly open: AddNameOpen | null;
  // name is the task's, under the title
  readonly name: string;
  // refusal is why what was typed (trimmed) is not added, '' to add it
  readonly refusal: (typed: string) => string;
  readonly onAdd: (typed: string) => void;
  readonly onClosed: () => void;
}

// AddNameDialog asks a URL or a category for the modal it is opened over (Edit's, Add a page's), which takes it and
// sends it with its own write: one field, Cancel and Add. A refusal is said under the field, the modal open; once added,
// the caller closes it. It stands beside that modal, not in it: in it, its Escape would reach that modal's cancel too,
// and its form would be nested in Edit's, sending both. id names its parts, as one stands for each modal
export function AddNameDialog({ id, open, name, refusal, onAdd, onClosed }: AddNameDialogProps) {
  // The last opened, which the modal shows and goes back from as it closes (open is null by then), and how many times
  // it was opened: its field is drawn anew, empty, for each
  const [opened, setOpened] = useState<{ last: AddNameOpen | null; n: number }>({ last: null, n: 0 });
  if (open !== null && open !== opened.last) setOpened({ last: open, n: opened.n + 1 });
  const added = useRef(false);
  return (
    <Dialog
      id={`${id}-dialog`}
      open={open !== null}
      onClosed={onClosed}
      labelledBy={`${id}-title`}
      afterClose={() => {
        const was = added.current;
        added.current = false;
        return opened.last?.back(was) ?? false;
      }}
    >
      <AddNameBody
        id={id}
        key={opened.n}
        url={opened.last?.kind !== 'category'}
        name={name}
        refusal={refusal}
        onAdd={(x) => {
          added.current = true;
          onAdd(x);
        }}
      />
    </Dialog>
  );
}

function AddNameBody({
  id,
  url,
  name,
  refusal,
  onAdd,
}: Pick<AddNameDialogProps, 'id' | 'name' | 'refusal' | 'onAdd'> & { readonly url: boolean }) {
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
        if (why) focusOn(document.getElementById(`${id}-new`));
        else onAdd(x);
      }}
    >
      <DialogHead
        icon={<PlusIcon />}
        titleId={`${id}-title`}
        title={t(url ? 'ui_url_add' : 'ui_category_add')}
        lead={name}
        leadId={`${id}-target`}
      />
      <label data-testid="field" className={FIELD}>
        {t(url ? 'ui_task_url' : 'ui_category_name')}
        <input
          id={`${id}-new`}
          data-testid={url ? 'path' : 'text'}
          className={url ? `${TEXT} font-mono` : TEXT}
          autoComplete="off"
          spellCheck={url ? false : undefined}
          placeholder={url ? 'https://' : undefined}
          aria-describedby={`${id}-said`}
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
      <p id={`${id}-said`} aria-live="polite" className={SAID}>
        {said}
      </p>
      <DialogActions>
        <DialogClose words="ui_cancel" />
        <Button id={`${id}-submit`} kind="primary" type="submit">
          {t('ui_add')}
        </Button>
      </DialogActions>
    </form>
  );
}
