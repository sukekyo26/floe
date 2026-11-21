// backdropDown tells a press that went down on the backdrop (the dialog itself; its body fills it) of a modal not held:
// only such a press, let go on the backdrop too, closes it. A click lands on the dialog also when it went down in a
// field and up outside, or down while held and up once let go
export const backdropDown = (onDialog: boolean, held: boolean): boolean => onDialog && !held;
