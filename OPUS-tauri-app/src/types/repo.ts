export type RepoInstance = {
  name: string;
  worktree_path: string;
  repo_dir: string;
  missing: boolean;
  default_branch?: string;
  default_remote?: string;
};
