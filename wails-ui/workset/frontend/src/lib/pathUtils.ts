/**
 * Format a file path for display in the sidebar, truncating from the left
 * while preserving the filename and last 2-3 directories.
 * Always shows the full filename, even if it's very long.
 * 
 * @param path - The full file path to format
 * @param maxChars - Maximum character length for the formatted path (default: 40)
 * @returns The formatted path string
 * 
 * @example
 * formatPath('pkg/sessiond/client.go') 
 * // returns: 'pkg/sessiond/client.go' (fits entirely)
 * 
 * @example
 * formatPath('wails-ui/workset/frontend/src/lib/components/RepoDiff.svelte')
 * // returns: '.../src/lib/components/RepoDiff.svelte'
 * 
 * @example
 * formatPath('very-long-component-name-that-is-excessive.tsx')
 * // returns: 'very-long-component-name-that-is-excessive.tsx' (full filename)
 */
export function formatPath(path: string, maxChars: number = 40): string {
	if (path.length <= maxChars) return path;

	const parts = path.split('/');
	const filename = parts.pop() || '';

	// Always show the full filename, even if it's very long
	if (filename.length >= maxChars) {
		return filename;
	}

	// Build from end, adding directories until we hit the limit
	const visible = [filename];
	let currentLength = filename.length;

	for (let i = parts.length - 1; i >= 0; i--) {
		const dir = parts[i];
		// +5 accounts for ".../" (4 chars) + "/" separator (1 char) that will be added
		if (currentLength + dir.length + 5 <= maxChars) {
			visible.unshift(dir);
			currentLength += dir.length + 1;
		} else {
			// Only add "..." if we have room for it (4 chars for ".../")
			if (currentLength + 4 <= maxChars) {
				visible.unshift('...');
			}
			break;
		}
	}

	return visible.join('/');
}
