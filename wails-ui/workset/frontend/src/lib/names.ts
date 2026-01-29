/**
 * Nature-themed word list for workspace name generation.
 * These create memorable, distinguishable workspace names.
 */
const natureWords = [
  // Trees
  'oak', 'maple', 'cedar', 'pine', 'birch', 'willow', 'aspen', 'elm',
  // Water
  'river', 'stream', 'lake', 'creek', 'brook', 'delta', 'falls', 'spring',
  // Sky/Weather
  'aurora', 'thunder', 'storm', 'cloud', 'dawn', 'dusk', 'mist', 'frost',
  // Terrain
  'ridge', 'valley', 'mesa', 'cliff', 'canyon', 'grove', 'meadow', 'peak',
  // Elements
  'stone', 'ember', 'flint', 'quartz', 'slate', 'coral', 'amber', 'jade',
  // Animals
  'falcon', 'heron', 'wolf', 'bear', 'hawk', 'raven', 'fox', 'elk'
]

/**
 * Generate a workspace name from a repo name with a random nature suffix.
 * Example: platform → platform-maple
 */
export function generateWorkspaceName(repoName: string): string {
  const word = natureWords[Math.floor(Math.random() * natureWords.length)]
  return `${repoName}-${word}`
}

/**
 * Generate alternative workspace names for suggestions.
 */
export function generateAlternatives(repoName: string, count = 2): string[] {
  const shuffled = [...natureWords].sort(() => Math.random() - 0.5)
  return shuffled.slice(0, count).map(word => `${repoName}-${word}`)
}

/**
 * Check if input looks like a Git URL.
 */
export function looksLikeUrl(input: string): boolean {
  const trimmed = input.trim()
  return (
    trimmed.startsWith('git@') ||
    trimmed.startsWith('https://') ||
    trimmed.startsWith('http://') ||
    trimmed.startsWith('ssh://') ||
    trimmed.includes('github.com') ||
    trimmed.includes('gitlab.com') ||
    trimmed.includes('bitbucket.org')
  )
}

/**
 * Check if input looks like a file system path.
 */
export function looksLikePath(input: string): boolean {
  const trimmed = input.trim()
  return (
    trimmed.startsWith('/') ||
    trimmed.startsWith('~') ||
    trimmed.startsWith('./') ||
    /^[A-Za-z]:[\\/]/.test(trimmed)  // Windows paths
  )
}

/**
 * Derive a repo name from a URL or path.
 * Returns null if the input is empty or can't be parsed.
 *
 * Examples:
 *   git@github.com:org/repo.git → repo
 *   https://github.com/org/repo → repo
 *   /Users/sean/src/worker → worker
 */
export function deriveRepoName(source: string): string | null {
  const trimmed = source.trim()
  if (!trimmed) return null

  // Handle URLs: git@github.com:org/repo.git → repo
  // Handle URLs: https://github.com/org/repo → repo
  let cleaned = trimmed.replace(/\.git$/, '')
  cleaned = cleaned.replace(/\/+$/, '')

  // SSH style: git@host:org/repo
  const sshMatch = cleaned.match(/:([^\/]+)$/)
  if (sshMatch && cleaned.includes('@')) {
    return sshMatch[1]
  }

  // HTTPS/path style: last segment
  const parts = cleaned.split('/').filter(Boolean)
  if (parts.length > 0) {
    return parts[parts.length - 1]
  }

  return null
}

/**
 * Check if input is a URL or local path (vs a plain name).
 */
export function isRepoSource(input: string): boolean {
  return looksLikeUrl(input) || looksLikePath(input)
}

/**
 * Tech-themed suffixes for terminal name generation.
 * These create fun, memorable terminal names that blend nature + tech.
 */
const techSuffixes = [
  'byte', 'buffer', 'stack', 'cache', 'thread',
  'kernel', 'node', 'packet', 'grid', 'core',
  'flux', 'signal', 'stream', 'pulse', 'spark',
  'loop', 'wire', 'link', 'seed', 'root'
]

/**
 * Extract the nature word from a workspace name.
 * Example: "platform-oak" → "oak"
 */
function extractNatureWord(workspaceName: string): string | null {
  const parts = workspaceName.split('-')
  const lastPart = parts[parts.length - 1]
  
  // Check if the last part is a nature word
  if (natureWords.includes(lastPart.toLowerCase())) {
    return lastPart.toLowerCase()
  }
  
  // Otherwise, return a random nature word
  return natureWords[Math.floor(Math.random() * natureWords.length)]
}

/**
 * Generate a unique terminal name based on the workspace.
 * Combines the workspace's nature word with a tech suffix.
 * Example: "oak-byte", "thunder-kernel", "stream-node"
 */
export function generateTerminalName(workspaceName: string, index: number = 0): string {
  const natureWord = extractNatureWord(workspaceName) || 'crystal'
  const techWord = techSuffixes[index % techSuffixes.length]
  return `${natureWord}-${techWord}`
}
