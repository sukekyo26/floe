// composing tells a key that goes to the input method: 229 as Safari sends its Enter that ends one without isComposing
export const composing = (e: Pick<KeyboardEvent, 'isComposing' | 'keyCode'>): boolean =>
  // oxlint-disable-next-line typescript/no-deprecated -- Safari tells its Enter that ends a composition by keyCode alone
  e.isComposing || e.keyCode === 229;
