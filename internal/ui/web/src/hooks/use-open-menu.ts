import { useEffect } from 'react';

import { menus as pageMenus, type Menus } from '@/stores/menus';
import { menusToClose } from '@/utils/menu';

// closeMenu closes the menu id, the focus going back to its button if it was in it or nowhere (on the body), rather than
// to the page: an item that opened a modal has moved it there
export function closeMenu(menus: Menus, id: string): void {
  const box = document.querySelector(`[data-menu-id="${CSS.escape(id)}"]`);
  const active = document.activeElement;
  const back = box !== null && (box.contains(active) || active === document.body);
  menus.hide(id);
  if (back) box.querySelector('summary')?.focus();
}

// useOpenMenus closes the menu open as a click lands elsewhere or on an item done (a delete's first press keeps it),
// on Escape, and as the window loses the focus (a press in a page, which this page is not told of). One per screen
export function useOpenMenus(menus: Menus = pageMenus): void {
  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      const target = e.target instanceof Element ? e.target : null;
      const item = target?.closest('[data-menu-item]') ?? null;
      const id = menusToClose(menus.open.get(), {
        menu: target?.closest('[data-menu-id]')?.getAttribute('data-menu-id') ?? null,
        item: item !== null,
        keepsOpen: item?.hasAttribute('data-keeps-open') ?? false,
      });
      if (id !== null) closeMenu(menus, id);
    };
    const onKey = (e: KeyboardEvent) => {
      const id = menus.open.get();
      if (e.key === 'Escape' && id !== null) closeMenu(menus, id);
    };
    const onBlur = () => {
      const id = menus.open.get();
      if (id !== null) closeMenu(menus, id);
    };
    document.addEventListener('click', onClick);
    document.addEventListener('keydown', onKey);
    window.addEventListener('blur', onBlur);
    return () => {
      document.removeEventListener('click', onClick);
      document.removeEventListener('keydown', onKey);
      window.removeEventListener('blur', onBlur);
    };
  }, [menus]);
}
