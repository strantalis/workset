export type EnvSnapshot = {
  path: string;
  shell: string;
  home: string;
  ssh_auth_sock?: string;
  git_ssh_command?: string;
  git_askpass?: string;
  gh_config_dir?: string;
  gh_auth_summary?: string;
};

export type SessiondStatus = {
  running: boolean;
  version?: string;
  socket_path?: string;
  last_error?: string;
  last_restart?: string;
};

export type CliStatus = {
  available: boolean;
  path?: string;
  version?: string;
  error?: string;
};
