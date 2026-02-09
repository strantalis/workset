export type TerminalLayout = {
  version: number;
  root: LayoutNode;
  focused_pane_id?: string;
};

export type LayoutNode = PaneNode | SplitNode;

export type PaneNode = {
  kind: 'pane';
  id: string;
  tabs: LayoutTab[];
  active_tab_id?: string;
};

export type SplitNode = {
  kind: 'split';
  id: string;
  direction: 'row' | 'column';
  ratio: number;
  first: LayoutNode;
  second: LayoutNode;
};

export type LayoutTab = {
  id: string;
  terminal_id: string;
  title: string;
  kind: 'agent' | 'terminal' | 'diff';
  diff_repo?: string;
  diff_repo_path?: string;
  diff_file_path?: string;
  diff_prev_path?: string;
  diff_status?: string;
};
