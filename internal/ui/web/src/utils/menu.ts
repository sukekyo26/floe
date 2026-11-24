// Where a click landed, as the menus see it: in which menu (null: none), on one of its items, and on one that keeps it
// open (a delete's first press)
export interface MenuClick {
  menu: string | null;
  item: boolean;
  keepsOpen: boolean;
}

// menusToClose is the menu a click closes, of the one open: a click elsewhere closes it, and so does an item done
export function menusToClose(open: string | null, click: MenuClick): string | null {
  if (open === null) return null;
  if (click.menu !== open) return open;
  return click.item && !click.keepsOpen ? open : null;
}
