export type WorksetProfile = {
  id: string;
  name: string;
  repos: string[];
  workspace_ids: string[];
  defaults?: WorksetDefaults;
  created_at: string;
  updated_at: string;
};

export type WorksetDefaults = {
  base_branch?: string;
  default_remote?: string;
  workspace_root?: string;
};
