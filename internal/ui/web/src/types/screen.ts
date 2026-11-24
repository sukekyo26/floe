// Screen is the screen shown: the page list, the task list, a task's page (the task list's URL with task=), or a page
// whole. The one place a task's page is told from the task list is screenOf
export type Screen = 'list' | 'taskList' | 'taskPage' | 'frame';
