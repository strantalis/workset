/**
 * Nature-themed word list for workspace name generation.
 * These create memorable, distinguishable workspace names.
 */
const natureWords = [
	// Trees
	'oak',
	'maple',
	'cedar',
	'pine',
	'birch',
	'willow',
	'aspen',
	'elm',
	// Water
	'river',
	'stream',
	'lake',
	'creek',
	'brook',
	'delta',
	'falls',
	'spring',
	// Sky/Weather
	'aurora',
	'thunder',
	'storm',
	'cloud',
	'dawn',
	'dusk',
	'mist',
	'frost',
	// Terrain
	'ridge',
	'valley',
	'mesa',
	'cliff',
	'canyon',
	'grove',
	'meadow',
	'peak',
	// Elements
	'stone',
	'ember',
	'flint',
	'quartz',
	'slate',
	'coral',
	'amber',
	'jade',
	// Animals
	'falcon',
	'heron',
	'wolf',
	'bear',
	'hawk',
	'raven',
	'fox',
	'elk',
];

/**
 * Generate a workspace name from a repo name with a random nature suffix.
 * Example: platform → platform-maple
 */
export function generateWorkspaceName(repoName: string): string {
	const word = natureWords[Math.floor(Math.random() * natureWords.length)];
	return `${repoName}-${word}`;
}

/**
 * Generate alternative workspace names for suggestions.
 */
export function generateAlternatives(repoName: string, count = 2): string[] {
	const shuffled = [...natureWords].sort(() => Math.random() - 0.5);
	return shuffled.slice(0, count).map((word) => `${repoName}-${word}`);
}

/**
 * Check if input looks like a Git URL.
 * Validates hostname against allowed git hosts to prevent SSRF.
 */
export function looksLikeUrl(input: string): boolean {
	const trimmed = input.trim();

	// SSH URLs: git@host:org/repo
	if (trimmed.startsWith('git@')) {
		const match = trimmed.match(/^git@([^:]+):/);
		if (match) {
			return isAllowedHost(match[1]);
		}
		return false;
	}

	// Standard URL schemes
	if (
		trimmed.startsWith('https://') ||
		trimmed.startsWith('http://') ||
		trimmed.startsWith('ssh://')
	) {
		try {
			const url = new URL(trimmed);
			return isAllowedHost(url.hostname);
		} catch {
			return false;
		}
	}

	return false;
}

/**
 * Check if hostname is in the allowed list.
 * Supports exact matches and subdomains of allowed hosts.
 */
function isAllowedHost(hostname: string): boolean {
	const allowedHosts = ['github.com', 'gitlab.com', 'bitbucket.org'];

	const normalized = hostname.toLowerCase();

	// Exact match
	if (allowedHosts.includes(normalized)) {
		return true;
	}

	// Subdomain match (e.g., github.company.com)
	return allowedHosts.some((allowed) => normalized.endsWith('.' + allowed));
}

/**
 * Check if input looks like a file system path.
 */
export function looksLikePath(input: string): boolean {
	const trimmed = input.trim();
	return (
		trimmed.startsWith('/') ||
		trimmed.startsWith('~') ||
		trimmed.startsWith('./') ||
		/^[A-Za-z]:[\\/]/.test(trimmed) // Windows paths
	);
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
	const trimmed = source.trim();
	if (!trimmed) return null;

	// Handle URLs: git@github.com:org/repo.git → repo
	// Handle URLs: https://github.com/org/repo → repo
	let cleaned = trimmed.replace(/\.git$/, '');
	cleaned = cleaned.replace(/\/+$/, '');

	// SSH style: git@host:org/repo
	const sshMatch = cleaned.match(/:([^/]+)$/);
	if (sshMatch && cleaned.includes('@')) {
		return sshMatch[1];
	}

	// HTTPS/path style: last segment
	const parts = cleaned.split('/').filter(Boolean);
	if (parts.length > 0) {
		return parts[parts.length - 1];
	}

	return null;
}

/**
 * Check if input is a URL or local path (vs a plain name).
 */
export function isRepoSource(input: string): boolean {
	return looksLikeUrl(input) || looksLikePath(input);
}

/**
 * Truncate a long label by preserving both the start and end segments.
 * Example: data-security-platform-thunder -> data-security…platform-thunder
 */
export function ellipsisMiddle(value: string, maxLength = 28): string {
	if (maxLength <= 0) return '';
	if (value.length <= maxLength) return value;
	if (maxLength === 1) return '…';

	const visible = maxLength - 1;
	const left = Math.ceil(visible / 2);
	const right = Math.floor(visible / 2);
	return `${value.slice(0, left)}…${value.slice(-right)}`;
}

export type SidebarLabelLimits = {
	workspace: number;
	repo: number;
	ref: number;
};

const clamp = (value: number, min: number, max: number): number =>
	Math.min(max, Math.max(min, value));

/**
 * Derive dynamic truncation limits from current sidebar width.
 * Keeps middle truncation responsive as users resize the workspace sidebar.
 */
export function deriveSidebarLabelLimits(sidebarWidthPx: number): SidebarLabelLimits {
	const width = Number.isFinite(sidebarWidthPx) ? sidebarWidthPx : 0;

	return {
		workspace: clamp(Math.floor((width - 86) / 6.6), 12, 72),
		repo: clamp(Math.floor((width - 110) / 6.5), 12, 64),
		ref: clamp(Math.floor((width - 170) / 6.2), 12, 48),
	};
}

/**
 * Tech-themed suffixes for terminal name generation.
 * These create fun, memorable terminal names that blend nature + tech.
 */
const techSuffixes = [
	'byte',
	'buffer',
	'stack',
	'cache',
	'thread',
	'kernel',
	'node',
	'packet',
	'grid',
	'core',
	'flux',
	'signal',
	'stream',
	'pulse',
	'spark',
	'loop',
	'wire',
	'link',
	'seed',
	'root',
];

/**
 * Extract the nature word from a workspace name.
 * Example: "platform-oak" → "oak"
 */
function extractNatureWord(workspaceName: string): string | null {
	const parts = workspaceName.split('-');
	const lastPart = parts[parts.length - 1];

	// Check if the last part is a nature word
	if (natureWords.includes(lastPart.toLowerCase())) {
		return lastPart.toLowerCase();
	}

	// Otherwise, return a random nature word
	return natureWords[Math.floor(Math.random() * natureWords.length)];
}

/**
 * Generate a unique terminal name based on the workspace.
 * Combines the workspace's nature word with a tech suffix.
 * Example: "oak-byte", "thunder-kernel", "stream-node"
 */
export function generateTerminalName(workspaceName: string, index: number = 0): string {
	const natureWord = extractNatureWord(workspaceName) || 'crystal';
	const techWord = techSuffixes[index % techSuffixes.length];
	return `${natureWord}-${techWord}`;
}
