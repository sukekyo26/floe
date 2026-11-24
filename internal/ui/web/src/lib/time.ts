import { getBoot } from '@/config/boot';
import { t } from '@/lib/t';

const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 31536000],
  ['month', 2592000],
  ['day', 86400],
  ['hour', 3600],
  ['minute', 60],
];

// The formats of the reader's language, made once it is first asked for
let made: { lang: string; date: Intl.DateTimeFormat; ago: Intl.RelativeTimeFormat } | null = null;
function formats() {
  const { lang } = getBoot();
  if (made?.lang !== lang) {
    made = {
      lang,
      date: new Intl.DateTimeFormat(lang, { dateStyle: 'medium', timeStyle: 'short' }),
      ago: new Intl.RelativeTimeFormat(lang, { numeric: 'auto' }),
    };
  }
  return made;
}

// fmt is a time as the reader writes a date and time; a value that is no time (a row that has none) gives nothing
export const fmt = (iso: string): string => {
  const ms = Date.parse(iso);
  return Number.isNaN(ms) ? '' : formats().date.format(ms);
};

// ago is how long ago a time was, in its largest unit; a value that is no time (a row that has none, as a folder's)
// gives nothing, not "just now"
export function ago(iso: string): string {
  const ms = Date.parse(iso);
  if (Number.isNaN(ms)) return '';
  const s = (ms - Date.now()) / 1000;
  const unit = UNITS.find(([, n]) => Math.abs(s) >= n);
  return unit ? formats().ago.format(Math.round(s / unit[1]), unit[0]) : t('ui_just_now');
}
