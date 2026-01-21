import type {
  Alias,
  Group,
  GroupSummary,
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
  DeleteAlias,
  DeleteGroup,
  GetGroup,
  GetRepoDiff,
  GetRepoDiffSummary,
  GetRepoFileDiff,
  GetAgentAvailability,
  GetSettings,
  ListAliases,
  ListGroups,
  ListWorkspaceSnapshots,
  OpenDirectoryDialog,
  RemoveGroupMember,
  RemoveRepo,
  RemoveWorkspace,
  RenameWorkspace,
  UpdateAlias,
  UpdateGroup,
  UnarchiveWorkspace,
  UpdateRepoRemotes,
  SetDefaultSetting
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
  branch?: string
  baseRemote?: string
  baseBranch?: string
  writeRemote?: string
  writeBranch?: string
  dirty: boolean
  missing: boolean
}

type RepoDiffSnapshot = {
  patch: string
}

export async function fetchWorkspaces(includeArchived = false): Promise<Workspace[]> {
  const snapshots = await ListWorkspaceSnapshots(includeArchived)
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
      branch: repo.branch,
      baseRemote: repo.baseRemote,
      baseBranch: repo.baseBranch,
      writeRemote: repo.writeRemote,
      writeBranch: repo.writeBranch,
      ahead: 0,
      behind: 0,
      dirty: repo.dirty,
      missing: repo.missing,
      diff: {added: 0, removed: 0},
      files: []
    }))
  }))
}

export async function createWorkspace(name: string, path: string): Promise<WorkspaceCreateResponse> {
  return CreateWorkspace({name, path})
}

export async function openDirectoryDialog(
  title: string,
  defaultDirectory: string
): Promise<string> {
  return OpenDirectoryDialog(title, defaultDirectory)
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

export async function updateRepoRemotes(
  workspaceId: string,
  repoName: string,
  baseRemote: string,
  baseBranch: string,
  writeRemote: string,
  writeBranch: string
): Promise<void> {
  await UpdateRepoRemotes({workspaceId, repoName, baseRemote, baseBranch, writeRemote, writeBranch})
}

export async function listAliases(): Promise<Alias[]> {
  return ListAliases()
}

export async function createAlias(
  name: string,
  source: string,
  defaultBranch: string
): Promise<void> {
  await CreateAlias({name, source, defaultBranch})
}

export async function updateAlias(
  name: string,
  source: string,
  defaultBranch: string
): Promise<void> {
  await UpdateAlias({name, source, defaultBranch})
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
  repoName: string,
  baseRemote: string,
  baseBranch: string,
  writeRemote: string
): Promise<void> {
  await AddGroupMember({
    groupName,
    repoName,
    baseRemote,
    baseBranch,
    writeRemote
  })
}

export async function removeGroupMember(groupName: string, repoName: string): Promise<void> {
  await RemoveGroupMember({
    groupName,
    repoName,
    baseRemote: '',
    writeRemote: '',
    baseBranch: ''
  })
}

export async function applyGroup(workspaceId: string, groupName: string): Promise<void> {
  await ApplyGroup(workspaceId, groupName)
}

export async function fetchSettings(): Promise<SettingsSnapshot> {
  return GetSettings()
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
