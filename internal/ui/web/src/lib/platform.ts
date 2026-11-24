// isMac tells a Mac, where the key that goes with K is ⌘
export const isMac = (): boolean => navigator.platform.includes('Mac');

// ONE_COLUMN is the width that has nothing beside the list
export const ONE_COLUMN = '(max-width: 56rem)';
// NARROW is the width under which the frame's tree lies over the page, kept nowhere (index.css narrow)
export const NARROW = '(max-width: 40rem)';

// oneColumn tells the window that narrow now
export const oneColumn = (): boolean => window.matchMedia(ONE_COLUMN).matches;

// goTo leaves the page for url, as a link pressed would
export const goTo = (url: string): void => {
  window.location.assign(url);
};
