// isMac tells a Mac, where the key that goes with K is ⌘ (ui.html reads navigator.platform the same way)
export const isMac = (): boolean => navigator.platform.includes('Mac');

// ONE_COLUMN is the width that has nothing beside the list (ui.html oneColumn)
export const ONE_COLUMN = '(max-width: 56rem)';

// oneColumn tells the window that narrow now
export const oneColumn = (): boolean => window.matchMedia(ONE_COLUMN).matches;
