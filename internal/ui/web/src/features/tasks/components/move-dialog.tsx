import { useRef } from 'react';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead, useDialog } from '@/components/dialog';
import { TaskSetIcon } from '@/components/icons';
import { moveAsk as ask0, type MoveAsk } from '@/features/tasks/stores/move-ask';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';

// Yes closes the question as its close buttons do, the answer yes
function MoveYes({ onYes }: { readonly onYes: () => void }) {
  const { close } = useDialog();
  return (
    <Button
      id="move-yes"
      kind="primary"
      onClick={() => {
        onYes();
        close();
      }}
    >
      {t('ui_yes')}
    </Button>
  );
}

// MoveDialog is the move's question, over the modal that asked it: No (its close button, Escape, the backdrop) writes
// nothing. It answers as it closes, in the same task, so that what follows goes on at once
export function MoveDialog({ ask = ask0 }: { readonly ask?: MoveAsk }) {
  const asking = useStore(ask.asking);
  const yes = useRef(false);
  const answer = () => {
    ask.answer(yes.current);
    yes.current = false;
  };
  return (
    <Dialog
      id="move-dialog"
      open={asking !== null}
      // Closed from outside too (close() called on the element): No
      onClosed={answer}
      afterClose={() => {
        answer();
        return false;
      }}
      labelledBy="move-title"
      describedBy="move-lead"
    >
      <DialogHead
        icon={<TaskSetIcon />}
        titleId="move-title"
        title={t('ui_task_move_ask')}
        lead={asking?.lead ?? ''}
        leadId="move-lead"
      />
      <DialogActions>
        <DialogClose words="ui_no" focusOnOpen id="move-no" />
        <MoveYes
          onYes={() => {
            yes.current = true;
          }}
        />
      </DialogActions>
    </Dialog>
  );
}
