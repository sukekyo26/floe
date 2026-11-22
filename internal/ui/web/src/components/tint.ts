// The paint of a project's colour (ui.html .proj-tint, and the dot before .proj): --c is the project's own, which :root
// does not decide, so its tint is mixed outside :root with :root's colours (ui.md manage-colours, its named exception)
export const TINT_VARS =
  '[--tint-ink:color-mix(in_srgb,var(--c)_50%,var(--color-fg))] [--tint:color-mix(in_srgb,var(--c)_16%,var(--color-panel))]';
export const TINT_DOT = "before:size-1.75 before:flex-none before:rounded-full before:bg-(--c) before:content-['']";
