import { useMinute } from '@/hooks/use-minute';
import { ago } from '@/lib/time';

// Ago is how long ago iso was, told anew every minute on its own: a row drawn once (memo) keeps its time going
export function Ago({ iso }: { readonly iso: string }) {
  useMinute();
  return ago(iso);
}
