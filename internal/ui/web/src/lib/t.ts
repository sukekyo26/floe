import { getBoot } from '@/config/boot';
import type { MessageKey } from '@/types/messages.gen';
import { NONE } from '@/utils/list-view';

// t fills the reader's message key with args. Messages use only %s, %d, %[n]s and %%, as Go's fmt does; a key the
// table lacks is said as itself
export function t(key: MessageKey, ...args: (string | number)[]): string {
  const messages: Partial<Record<MessageKey, string>> = getBoot().messages;
  let next = 0;
  return (messages[key] ?? key).replace(/%(?:\[(\d+)\])?([sd%])/g, (_, n: string | undefined, verb: string) => {
    if (verb === '%') return '%';
    if (n) next = Number(n) - 1;
    return String(args[next++]);
  });
}

// projectWord is a project's name as the screen says it
export const projectWord = (name: string): string => (name === NONE ? t('ui_no_project') : name);
