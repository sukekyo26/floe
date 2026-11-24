import { t } from '@/lib/t';

// A version of a stored page, as GET /api/pages/{slug}/versions lists it
export interface Version {
  n: number;
  at: string;
}

// The versions of a page, oldest first, and the one read before the last read (0 for none)
export interface VersionList {
  versions: Version[];
  read: number;
}

// verMonth is a version's time as month/day hours:minutes, in the reader's time zone
export function verMonth(iso: string): string {
  const d = new Date(iso);
  const two = (x: number) => String(x).padStart(2, '0');
  return `${two(d.getMonth() + 1)}/${two(d.getDate())} ${two(d.getHours())}:${two(d.getMinutes())}`;
}

// verRest is what is told of v but its number: its time, and whether it is the latest or the one read before
export function verRest(v: Version, { versions, read }: VersionList): string {
  const note = v.n === versions.at(-1)?.n ? t('ui_diff_latest') : v.n === read ? t('ui_diff_read_version') : '';
  return [verMonth(v.at), note].filter(Boolean).join(' · ');
}
