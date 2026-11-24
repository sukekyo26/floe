import { verRest, type VersionList } from '@/features/versions/utils/version-words';

// The two versions compared, from older to later
export interface Pair {
  from: number;
  to: number;
}

const has = (list: VersionList, n: number | undefined) => list.versions.some((v) => v.n === n);

// fromFor is what to is compared with first: the version read before if it is older, else the one just before
export function fromFor(to: number, list: VersionList): number | undefined {
  if (has(list, list.read) && list.read < to) return list.read;
  return list.versions.filter((v) => v.n < to).at(-1)?.n;
}

// firstPair is the pair the URL's query (search) asks for, what it leaves out its default: to the latest, from as
// fromFor. One that cannot be compared gives way to the first pair, bad saying so
export function firstPair(search: string, list: VersionList): { pair: Pair; bad: boolean } {
  const q = new URLSearchParams(search);
  const asked = (name: string) => {
    const value = q.get(name);
    if (value === null) return undefined;
    return /^[1-9][0-9]*$/.test(value) ? Number(value) : Number.NaN;
  };
  const last = list.versions.at(-1)?.n ?? 0;
  const to = asked('to') ?? last;
  const from = asked('from') ?? fromFor(to, list);
  if (from !== undefined && has(list, from) && has(list, to) && from < to) return { pair: { from, to }, bad: false };
  return { pair: { from: fromFor(last, list) ?? 0, to: last }, bad: true };
}

// pairOptions are the options of the choice name, newest first: v{n} · its words, one that would not be older (from)
// or later (to) than the other not to be chosen
export function pairOptions(
  name: keyof Pair,
  pair: Pair,
  list: VersionList,
): { n: number; text: string; off: boolean }[] {
  return list.versions.toReversed().map((v) => ({
    n: v.n,
    text: `v${String(v.n)} · ${verRest(v, list)}`,
    off: name === 'from' ? v.n >= pair.to : v.n <= pair.from,
  }));
}
