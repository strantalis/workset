import type {RemoteInfo} from '../types'

export type PullRequestRefs = {
  baseRepo?: string
  baseBranch?: string
  headRepo?: string
  headBranch?: string
}

export const resolveRemoteRef = (
  remotes: RemoteInfo[],
  repoFullName?: string,
  branch?: string
): string | null => {
  if (!repoFullName || !branch) return null
  const trimmed = repoFullName.trim()
  const parts = trimmed.split('/')
  if (parts.length !== 2) return null
  const [owner, repo] = parts
  if (!owner || !repo) return null
  const remote = remotes.find(
    (entry) => entry.owner === owner && entry.repo === repo
  )
  if (!remote) return null
  return `${remote.name}/${branch}`
}

export const resolveBranchRefs = (
  remotes: RemoteInfo[],
  pr?: PullRequestRefs | null
): {base: string; head: string} | null => {
  if (!pr?.baseBranch || !pr?.headBranch) {
    return null
  }
  return {
    base: resolveRemoteRef(remotes, pr.baseRepo, pr.baseBranch) ?? pr.baseBranch,
    head: resolveRemoteRef(remotes, pr.headRepo, pr.headBranch) ?? pr.headBranch
  }
}
