/**
 * Parse a unified diff patch string to extract original and modified file content.
 *
 * This reconstructs the before/after content from a unified diff patch,
 * which is needed because @codemirror/merge expects full file contents
 * rather than a patch string.
 */

interface ParsedPatch {
	original: string;
	modified: string;
}

interface HunkHeader {
	oldStart: number;
	oldCount: number;
	newStart: number;
	newCount: number;
}

const parseHunkHeader = (line: string): HunkHeader | null => {
	const match = line.match(/^@@\s+-(\d+)(?:,(\d+))?\s+\+(\d+)(?:,(\d+))?\s+@@/);
	if (!match) return null;
	return {
		oldStart: parseInt(match[1], 10),
		oldCount: match[2] !== undefined ? parseInt(match[2], 10) : 1,
		newStart: parseInt(match[3], 10),
		newCount: match[4] !== undefined ? parseInt(match[4], 10) : 1,
	};
};

/**
 * Parse a unified diff patch into original and modified content.
 * Handles multi-hunk patches and no-newline-at-end-of-file markers.
 */
export const parsePatch = (patch: string): ParsedPatch => {
	const lines = patch.split('\n');
	const originalLines: string[] = [];
	const modifiedLines: string[] = [];

	let inHunk = false;

	for (const line of lines) {
		// Skip diff headers
		if (
			line.startsWith('diff ') ||
			line.startsWith('index ') ||
			line.startsWith('---') ||
			line.startsWith('+++')
		) {
			continue;
		}

		// Hunk header
		if (line.startsWith('@@')) {
			const header = parseHunkHeader(line);
			if (header) {
				inHunk = true;
				// Pad with empty lines if there's a gap between hunks
				while (originalLines.length < header.oldStart - 1) {
					originalLines.push('');
				}
				while (modifiedLines.length < header.newStart - 1) {
					modifiedLines.push('');
				}
			}
			continue;
		}

		if (!inHunk) continue;

		// Skip no-newline marker
		if (line.startsWith('\\ No newline')) continue;

		if (line.startsWith('-')) {
			originalLines.push(line.slice(1));
		} else if (line.startsWith('+')) {
			modifiedLines.push(line.slice(1));
		} else {
			// Context line (starts with space or is empty)
			const content = line.startsWith(' ') ? line.slice(1) : line;
			originalLines.push(content);
			modifiedLines.push(content);
		}
	}

	return {
		original: originalLines.join('\n'),
		modified: modifiedLines.join('\n'),
	};
};
