import { NONE } from '@/utils/list-view';
import type { Page } from '@/types/page';

// projectOrder places the projects of every entry of items (of any kind, archived or not), most entries first, then by
// name: the order of the projects on the shelf and of their colors, which no filter changes. names, of what else is
// drawn, come after them at 0
export function projectOrder(items: readonly Page[], names: Iterable<string> = []): Map<string, number> {
  const counts = new Map<string, number>();
  for (const a of items) if (a.project) counts.set(a.project, (counts.get(a.project) ?? 0) + 1);
  for (const name of names) if (name !== NONE && !counts.has(name)) counts.set(name, 0);
  const count = (name: string) => counts.get(name) ?? 0;
  return new Map(
    [...counts.keys()].sort((x, y) => count(y) - count(x) || x.localeCompare(y)).map((name, i) => [name, i]),
  );
}
