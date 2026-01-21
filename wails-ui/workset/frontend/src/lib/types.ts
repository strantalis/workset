export type DiffFile = {
  path: string
  added: number
  removed: number
  hunks: string[]
}

export type Repo = {
  id: string
  name: string
  path: string
  branch?: string
  baseRemote?: string
  baseBranch?: string
  writeRemote?: string
  writeBranch?: string
  ahead?: number
  behind?: number
  dirty: boolean
  missing: boolean
  diff: {
    added: number
    removed: number
  }
  files: DiffFile[]
}

export type Workspace = {
  id: string
  name: string
  path: string
  archived: boolean
  archivedAt?: string
  archivedReason?: string
  repos: Repo[]
}

export type WorkspaceCreateResponse = {
  workspace: {
    name: string
    path: string
    workset: string
    branch: string
    next: string
  }
  warnings?: string[]
  pendingHooks?: {event: string; repo: string; hooks: string[]; status?: string; reason?: string}[]
}

export type RepoAddResponse = {
  payload: {
    status: string
    workspace: string
    repo: string
    local_path: string
    managed: boolean
    pending_hooks?: {event: string; repo: string; hooks: string[]; status?: string; reason?: string}[]
  }
  warnings?: string[]
  pendingHooks?: {event: string; repo: string; hooks: string[]; status?: string; reason?: string}[]
}

export type Alias = {
  name: string
  url?: string
  path?: string
  default_branch?: string
}

export type GroupSummary = {
  name: string
  description?: string
  repo_count: number
}

export type GroupMember = {
  repo: string
  remotes: {
    base: {name: string; default_branch?: string}
    write: {name: string; default_branch?: string}
  }
}

export type Group = {
  name: string
  description?: string
  members: GroupMember[]
}

export type SettingsDefaults = {
  baseBranch: string
  workspace: string
  workspaceRoot: string
  repoStoreRoot: string
  sessionBackend: string
  sessionNameFormat: string
  sessionTheme: string
  sessionTmuxStyle: string
  sessionTmuxLeft: string
  sessionTmuxRight: string
  sessionScreenHard: string
  agent: string
}

export type SettingsSnapshot = {
  defaults: SettingsDefaults
  configPath: string
}

export type RepoDiffFileSummary = {
  path: string
  prevPath?: string
  added: number
  removed: number
  status: string
  binary?: boolean
}

export type RepoDiffSummary = {
  files: RepoDiffFileSummary[]
  totalAdded: number
  totalRemoved: number
}

export type RepoFileDiff = {
  patch: string
  truncated: boolean
  totalBytes: number
  totalLines: number
  binary?: boolean
}
