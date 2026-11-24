// The paint of a project's colour: --c is the project's own, which @theme does not decide, so its tint is mixed
// outside @theme with @theme's colours (ui.md: the colours' one exception)
export const TINT_VARS =
  '[--tint-ink:color-mix(in_srgb,var(--c)_50%,var(--color-fg))] [--tint:color-mix(in_srgb,var(--c)_16%,var(--color-panel))]';
export const TINT_DOT = "before:size-1.75 before:flex-none before:rounded-full before:bg-(--c) before:content-['']";
