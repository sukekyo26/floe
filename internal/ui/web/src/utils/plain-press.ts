// A press as the mouse event tells it
export type Press = Pick<MouseEvent, 'button' | 'ctrlKey' | 'metaKey' | 'shiftKey' | 'altKey'>;

// plainPress tells a press of the main button with no modifier: any other is the link's, in a tab of its own
export const plainPress = (e: Press): boolean => e.button === 0 && !e.ctrlKey && !e.metaKey && !e.shiftKey && !e.altKey;
