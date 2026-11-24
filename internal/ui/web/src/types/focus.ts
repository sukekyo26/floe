// FocusBack gives the focus back where a closed modal or a done action leaves it, telling whether there was one to take
// it; it finds its place when called (a row's menu drawn anew, say)
export type FocusBack = () => boolean;
