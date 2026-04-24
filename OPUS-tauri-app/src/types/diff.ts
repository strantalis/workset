export type DiffSummary = {
  files: DiffFileSummary[];
  total_added: number;
  total_removed: number;
};

export type DiffFileSummary = {
  path: string;
  prev_path?: string;
  added: number;
  removed: number;
  status: 'M' | 'A' | 'D' | 'R';
  binary?: boolean;
};

export type FilePatch = {
  patch: string;
  truncated: boolean;
  total_bytes: number;
  total_lines: number;
  binary?: boolean;
};
