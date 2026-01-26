import type {
  Alias,
  Group,
  GroupSummary,
  PullRequestCheck,
  PullRequestCreated,
  PullRequestGenerated,
  PullRequestReviewComment,
  PullRequestStatusResult,
  RepoAddResponse,
  RepoDiffSummary,
  RepoFileDiff,
  SettingsSnapshot,
  Workspace,
  WorkspaceCreateResponse
} from './types'
import {
  AddGroupMember,
  AddRepo,
  ApplyGroup,
  ArchiveWorkspace,
  CreateWorkspace,
  CreateAlias,
  CreateGroup,
  CommitAndPush,
  DeleteAlias,
  DeleteGroup,
  GetGroup,
  GetRepoDiff,
  GetRepoDiffSummary,
  GetRepoFileDiff,
  GetRepoLocalStatus,
  GetBranchDiffSummary,
  GetBranchFileDiff,
  GetPullRequestReviews,
  GetPullRequestStatus,
  GetTrackedPullRequest,
  GetAgentAvailability,
  GeneratePullRequestText,
  GetSettings,
  GetSessiondStatus,
  RestartSessiond,
  ListAliases,
  ListGroups,
  ListWorkspaceSnapshots,
  OpenDirectoryDialog,
  RemoveGroupMember,
  RemoveRepo,
  RemoveWorkspace,
  RenameWorkspace,
  SendPullRequestReviewsToTerminal,
  UpdateAlias,
  UpdateGroup,
  UnarchiveWorkspace,
  SetDefaultSetting,
  CreatePullRequest,
  GetTerminalBacklog,
  GetTerminalBootstrap,
  GetTerminalSnapshot,
  LogTerminalDebug,
  GetWorkspaceTerminalStatus
} from '../../wailsjs/go/main/App'

type WorkspaceSnapshot = {
  id: string
  name: string
  path: string
  archived: boolean
  archivedAt?: string
  archivedReason?: string
  repos: RepoSnapshot[]
}

type RepoSnapshot = {
  id: string
  name: string
  path: string
  remote?: string
  defaultBranch?: string
  dirty: boolean
  missing: boolean
  statusKnown: boolean
}

type RepoDiffSnapshot = {
  patch: string
}

type PullRequestStatusResponse = {
  pullRequest: {
    repo: string
    number: number
    url: string
    title: string
    state: string
    draft: boolean
    base_repo: string
    base_branch: string
    head_repo: string
    head_branch: string
    mergeable?: string
  }
  checks: Array<{
    name: string
    status: string
    conclusion?: string
    details_url?: string
    started_at?: string
    completed_at?: string
  }>
}

type PullRequestCreateResponse = {
  repo: string
  number: number
  url: string
  title: string
  body?: string
  draft: boolean
  state: string
  base_repo: string
  base_branch: string
  head_repo: string
  head_branch: string
}

type PullRequestReviewCommentResponse = {
  id: number
  review_id?: number
  author?: string
  body: string
  path: string
  line?: number
  side?: string
  commit_id?: string
  original_commit_id?: string
  original_line?: number
  original_start_line?: number
  outdated: boolean
  url?: string
  created_at?: string
  updated_at?: string
  in_reply_to?: number
  reply?: boolean
}

export type RepoLocalStatus = {
  hasUncommitted: boolean
  ahead: number
  behind: number
  currentBranch: string
}

export type CommitAndPushResult = {
  committed: boolean
  pushed: boolean
  message: string
  sha?: string
}

export type TerminalBacklogResponse = {
  workspaceId: string
  data: string
  nextOffset: number
  truncated: boolean
  source?: string
}

export type TerminalSnapshotResponse = {
  workspaceId: string
  data: string
  source?: string
  kitty?: {
    images?: Array<{
      id: string
      format?: string
      width?: number
      height?: number
      data?: string | number[]
    }>
    placements?: Array<{
      id: number
      imageId: string
      row: number
      col: number
      rows: number
      cols: number
      x?: number
      y?: number
      z?: number
    }>
  }
}

export type TerminalBootstrapResponse = {
  workspaceId: string
  snapshot?: string
  snapshotSource?: string
  kitty?: {
    images?: Array<{
      id: string
      format?: string
      width?: number
      height?: number
      data?: string | number[]
    }>
    placements?: Array<{
      id: number
      imageId: string
      row: number
      col: number
      rows: number
      cols: number
      x?: number
      y?: number
      z?: number
    }>
  }
  backlog?: string
  backlogSource?: string
  backlogTruncated?: boolean
  nextOffset?: number
  source?: string
  altScreen?: boolean
  mouse?: boolean
  mouseSGR?: boolean
  mouseEncoding?: string
  safeToReplay?: boolean
}

export type SessiondStatusResponse = {
  available: boolean
  error?: string
  warning?: string
}

export type WorkspaceTerminalStatusResponse = {
  workspaceId: string
  active: boolean
  error?: string
}

export async function fetchWorkspaces(
  includeArchived = false,
  includeStatus = false
): Promise<Workspace[]> {
  const snapshots = await ListWorkspaceSnapshots({includeArchived, includeStatus})
  return snapshots.map((workspace: WorkspaceSnapshot) => ({
    id: workspace.id,
    name: workspace.name,
    path: workspace.path,
    archived: workspace.archived,
    archivedAt: workspace.archivedAt,
    archivedReason: workspace.archivedReason,
    repos: workspace.repos.map((repo: RepoSnapshot) => ({
      id: repo.id,
      name: repo.name,
      path: repo.path,
      remote: repo.remote,
      defaultBranch: repo.defaultBranch,
      ahead: 0,
      behind: 0,
      dirty: repo.dirty,
      missing: repo.missing,
      statusKnown: repo.statusKnown,
      diff: {added: 0, removed: 0},
      files: []
    }))
  }))
}

export async function createWorkspace(
  name: string,
  path: string,
  aliases?: string[],
  groups?: string[]
): Promise<WorkspaceCreateResponse> {
  return CreateWorkspace({
    name,
    path,
    repos: aliases,
    groups
  })
}

export async function openDirectoryDialog(
  title: string,
  defaultDirectory: string
): Promise<string> {
  return OpenDirectoryDialog(title, defaultDirectory)
}

export async function fetchWorkspaceTerminalStatus(
  workspaceId: string
): Promise<WorkspaceTerminalStatusResponse> {
  return GetWorkspaceTerminalStatus(workspaceId)
}

export async function fetchTerminalSnapshot(
  workspaceId: string
): Promise<TerminalSnapshotResponse> {
  return GetTerminalSnapshot(workspaceId)
}

export async function fetchTerminalBootstrap(
  workspaceId: string
): Promise<TerminalBootstrapResponse> {
  return GetTerminalBootstrap(workspaceId)
}

export async function logTerminalDebug(
  workspaceId: string,
  event: string,
  details = ''
): Promise<void> {
  await LogTerminalDebug({workspaceId, event, details})
}

export async function renameWorkspace(workspaceId: string, newName: string): Promise<void> {
  await RenameWorkspace(workspaceId, newName)
}

export async function archiveWorkspace(workspaceId: string, reason: string): Promise<void> {
  await ArchiveWorkspace(workspaceId, reason)
}

export async function unarchiveWorkspace(workspaceId: string): Promise<void> {
  await UnarchiveWorkspace(workspaceId)
}

export async function removeWorkspace(workspaceId: string): Promise<void> {
  await RemoveWorkspace(workspaceId)
}

export async function addRepo(
  workspaceId: string,
  source: string,
  name: string,
  repoDir: string
): Promise<RepoAddResponse> {
  return AddRepo({workspaceId, source, name, repoDir})
}

export async function removeRepo(
  workspaceId: string,
  repoName: string,
  deleteWorktree: boolean,
  deleteLocal: boolean
): Promise<void> {
  await RemoveRepo({workspaceId, repoName, deleteWorktree, deleteLocal})
}

export async function fetchTerminalBacklog(
  workspaceId: string,
  since: number
): Promise<TerminalBacklogResponse> {
  return GetTerminalBacklog(workspaceId, since)
}

export async function fetchSessiondStatus(): Promise<SessiondStatusResponse> {
  return GetSessiondStatus()
}

export async function restartSessiond(): Promise<SessiondStatusResponse> {
  return RestartSessiond()
}

export async function listAliases(): Promise<Alias[]> {
  return ListAliases()
}

export async function createAlias(
  name: string,
  source: string,
  remote: string,
  defaultBranch: string
): Promise<void> {
  await CreateAlias({name, source, remote, defaultBranch})
}

export async function updateAlias(
  name: string,
  source: string,
  remote: string,
  defaultBranch: string
): Promise<void> {
  await UpdateAlias({name, source, remote, defaultBranch})
}

export async function deleteAlias(name: string): Promise<void> {
  await DeleteAlias(name)
}

export async function listGroups(): Promise<GroupSummary[]> {
  return ListGroups()
}

export async function getGroup(name: string): Promise<Group> {
  return GetGroup(name)
}

export async function createGroup(name: string, description: string): Promise<void> {
  await CreateGroup({name, description})
}

export async function updateGroup(name: string, description: string): Promise<void> {
  await UpdateGroup({name, description})
}

export async function deleteGroup(name: string): Promise<void> {
  await DeleteGroup(name)
}

export async function addGroupMember(
  groupName: string,
  repoName: string
): Promise<void> {
  await AddGroupMember({
    groupName,
    repoName
  })
}

export async function removeGroupMember(groupName: string, repoName: string): Promise<void> {
  await RemoveGroupMember({
    groupName,
    repoName
  })
}

export async function applyGroup(workspaceId: string, groupName: string): Promise<void> {
  await ApplyGroup(workspaceId, groupName)
}

export async function fetchSettings(): Promise<SettingsSnapshot> {
  return (await GetSettings()) as unknown as SettingsSnapshot
}

export async function fetchAgentAvailability(): Promise<Record<string, boolean>> {
  return GetAgentAvailability()
}

export async function setDefaultSetting(key: string, value: string): Promise<void> {
  await SetDefaultSetting(key, value)
}

export async function fetchRepoDiff(
  workspaceId: string,
  repoId: string
): Promise<RepoDiffSnapshot> {
  return GetRepoDiff(workspaceId, repoId)
}

export async function fetchRepoDiffSummary(
  workspaceId: string,
  repoId: string
): Promise<RepoDiffSummary> {
  return GetRepoDiffSummary(workspaceId, repoId)
}

export async function fetchRepoFileDiff(
  workspaceId: string,
  repoId: string,
  path: string,
  prevPath: string,
  status: string
): Promise<RepoFileDiff> {
  return GetRepoFileDiff(workspaceId, repoId, path, prevPath, status)
}

export async function fetchBranchDiffSummary(
  workspaceId: string,
  repoId: string,
  base: string,
  head: string
): Promise<RepoDiffSummary> {
  return GetBranchDiffSummary(workspaceId, repoId, base, head)
}

export async function fetchBranchFileDiff(
  workspaceId: string,
  repoId: string,
  base: string,
  head: string,
  path: string,
  prevPath: string
): Promise<RepoFileDiff> {
  return GetBranchFileDiff(workspaceId, repoId, base, head, path, prevPath)
}

export async function createPullRequest(
  workspaceId: string,
  repoId: string,
  payload: {
    title: string
    body: string
    base?: string
    head?: string
    draft: boolean
    autoCommit?: boolean
    autoPush?: boolean
  }
): Promise<PullRequestCreated> {
  const result = (await CreatePullRequest({
    workspaceId,
    repoId,
    title: payload.title,
    body: payload.body,
    base: payload.base ?? '',
    head: payload.head ?? '',
    draft: payload.draft,
    autoCommit: payload.autoCommit ?? false,
    autoPush: payload.autoPush ?? false
  })) as PullRequestCreateResponse
  return mapPullRequest(result)
}

export async function fetchTrackedPullRequest(
  workspaceId: string,
  repoId: string
): Promise<PullRequestCreated | null> {
  const result = (await GetTrackedPullRequest({
    workspaceId,
    repoId
  })) as unknown as {found: boolean; pull_request?: PullRequestCreateResponse}
  if (!result.found || !result.pull_request) {
    return null
  }
  return mapPullRequest(result.pull_request)
}

export async function fetchPullRequestStatus(
  workspaceId: string,
  repoId: string,
  number?: number,
  branch?: string
): Promise<PullRequestStatusResult> {
  const result = (await GetPullRequestStatus({
    workspaceId,
    repoId,
    number: number ?? 0,
    branch: branch ?? ''
  })) as unknown as PullRequestStatusResponse
  const checks: PullRequestCheck[] = (result.checks ?? []).map((check) => ({
    name: check.name,
    status: check.status,
    conclusion: check.conclusion,
    detailsUrl: check.details_url,
    startedAt: check.started_at,
    completedAt: check.completed_at
  }))
  return {
    pullRequest: {
      repo: result.pullRequest.repo,
      number: result.pullRequest.number,
      url: result.pullRequest.url,
      title: result.pullRequest.title,
      state: result.pullRequest.state,
      draft: result.pullRequest.draft,
      baseRepo: result.pullRequest.base_repo,
      baseBranch: result.pullRequest.base_branch,
      headRepo: result.pullRequest.head_repo,
      headBranch: result.pullRequest.head_branch,
      mergeable: result.pullRequest.mergeable
    },
    checks
  }
}

function mapPullRequest(result: PullRequestCreateResponse): PullRequestCreated {
  return {
    repo: result.repo,
    number: result.number,
    url: result.url,
    title: result.title,
    body: result.body,
    draft: result.draft,
    state: result.state,
    baseRepo: result.base_repo,
    baseBranch: result.base_branch,
    headRepo: result.head_repo,
    headBranch: result.head_branch
  }
}

export async function fetchPullRequestReviews(
  workspaceId: string,
  repoId: string,
  number?: number,
  branch?: string
): Promise<PullRequestReviewComment[]> {
  const result = (await GetPullRequestReviews({
    workspaceId,
    repoId,
    number: number ?? 0,
    branch: branch ?? ''
  })) as unknown as {comments: PullRequestReviewCommentResponse[]}
  return (result.comments ?? []).map((comment) => ({
    id: comment.id,
    reviewId: comment.review_id,
    author: comment.author,
    body: comment.body,
    path: comment.path,
    line: comment.line,
    side: comment.side,
    commitId: comment.commit_id,
    originalCommit: comment.original_commit_id,
    originalLine: comment.original_line,
    originalStart: comment.original_start_line,
    outdated: comment.outdated,
    url: comment.url,
    createdAt: comment.created_at,
    updatedAt: comment.updated_at,
    inReplyTo: comment.in_reply_to,
    reply: comment.reply
  }))
}

export async function generatePullRequestText(
  workspaceId: string,
  repoId: string
): Promise<PullRequestGenerated> {
  const result = (await GeneratePullRequestText({
    workspaceId,
    repoId
  })) as PullRequestGenerated
  return result
}

export async function sendPullRequestReviewsToTerminal(
  workspaceId: string,
  repoId: string,
  number?: number,
  branch?: string
): Promise<void> {
  await SendPullRequestReviewsToTerminal({
    workspaceId,
    repoId,
    number: number ?? 0,
    branch: branch ?? ''
  })
}

export async function fetchRepoLocalStatus(
  workspaceId: string,
  repoId: string
): Promise<RepoLocalStatus> {
  const result = (await GetRepoLocalStatus({
    workspaceId,
    repoId
  })) as RepoLocalStatus
  return result
}

export async function commitAndPush(
  workspaceId: string,
  repoId: string,
  message?: string
): Promise<CommitAndPushResult> {
  const result = (await CommitAndPush({
    workspaceId,
    repoId,
    message: message ?? ''
  })) as CommitAndPushResult
  return result
}
