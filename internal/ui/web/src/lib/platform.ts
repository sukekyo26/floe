// isMac tells a Mac, where the key that goes with K is ⌘ (ui.html reads navigator.platform the same way)
export const isMac = (): boolean => navigator.platform.includes('Mac');
