export type PtyCreateResult = { terminal_id: string };

export type BootstrapPayload = {
  workspace_name: string;
  terminal_id: string;
  snapshot?: string;
  backlog?: string;
  backlog_truncated?: boolean;
  next_offset?: number;
  alt_screen?: boolean;
  mouse?: boolean;
  mouse_sgr?: boolean;
  safe_to_replay?: boolean;
  initial_credit?: number;
};
