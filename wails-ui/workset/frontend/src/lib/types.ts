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
  remote?: string
  defaultBranch?: string
  ahead?: number
  behind?: number
  dirty: boolean
  missing: boolean
  statusKnown?: boolean
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
  remote?: string
  default_branch?: string
}

export type GroupSummary = {
  name: string
  description?: string
  repo_count: number
}

export type GroupMember = {
  repo: string
}

export type Group = {
  name: string
  description?: string
  members: GroupMember[]
}

export type SettingsDefaults = {
  remote: string
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
  terminalRenderer: string
  terminalIdleTimeout: string
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

export type PullRequestSummary = {
  repo: string
  number: number
  url: string
  title: string
  body?: string
  state: string
  draft: boolean
  baseRepo: string
  baseBranch: string
  headRepo: string
  headBranch: string
  mergeable?: string
}

export type PullRequestCheck = {
  name: string
  status: string
  conclusion?: string
  detailsUrl?: string
  startedAt?: string
  completedAt?: string
}

export type PullRequestStatusResult = {
  pullRequest: PullRequestSummary
  checks: PullRequestCheck[]
}

export type PullRequestCreated = PullRequestSummary

export type PullRequestReviewComment = {
  id: number
  nodeId?: string
  threadId?: string
  reviewId?: number
  author?: string
  authorId?: number
  body: string
  path: string
  line?: number
  side?: string
  commitId?: string
  originalCommit?: string
  originalLine?: number
  originalStart?: number
  outdated: boolean
  url?: string
  createdAt?: string
  updatedAt?: string
  inReplyTo?: number
  reply?: boolean
  resolved?: boolean
}

export type PullRequestGenerated = {
  title: string
  body: string
}

export type RemoteInfo = {
  name: string
  owner: string
  repo: string
}
