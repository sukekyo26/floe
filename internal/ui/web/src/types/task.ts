// A state a task can be in, the server's one table (store.States)
export interface State {
  name: string;
  new: boolean;
  first: boolean;
}

// A task's row of tasks.json (store.Task)
export interface TaskRow {
  title?: string;
  state: string;
  at: string;
  urls?: string[];
  categories?: string[];
  parent?: string;
  rev?: number;
}

// tasks.json: the rows by project (as tasks.json keys it), then by name
export type Tasks = Record<string, Record<string, TaskRow>>;

// A task a write of pages and their tasks touched, as the write answers
export interface TaskWritten {
  taskProject: string;
  task: string;
  row: TaskRow;
}
