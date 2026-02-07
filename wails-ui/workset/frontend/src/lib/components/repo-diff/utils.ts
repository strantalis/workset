import type { GitHubOperationStage } from '../../api/github';

/**
 * Open URL only if it belongs to trusted GitHub domains.
 */
export function openTrustedGitHubURL(
	url: string | undefined | null,
	openURL: (value: string) => void,
): void {
	if (!url) return;

	try {
		const parsed = new URL(url);
		const hostname = parsed.hostname.toLowerCase();
		if (hostname === 'github.com' || hostname.endsWith('.github.com')) {
			openURL(url);
		}
	} catch {
		// Ignore invalid URLs.
	}
}

export function formatRepoDiffError(err: unknown, fallback: string): string {
	if (err instanceof Error) return err.message;
	if (typeof err === 'string') return err;
	if (err && typeof err === 'object' && 'message' in err) {
		const message = (err as { message?: string }).message;
		if (typeof message === 'string') return message;
	}
	return fallback;
}

export function parseOptionalNumber(value: string): number | undefined {
	const parsed = Number.parseInt(value.trim(), 10);
	return Number.isFinite(parsed) ? parsed : undefined;
}

export function getCommitPushStageCopy(stage: GitHubOperationStage | null): string {
	switch (stage) {
		case 'queued':
			return 'Preparing...';
		case 'generating_message':
			return 'Generating message...';
		case 'staging':
			return 'Staging changes...';
		case 'committing':
			return 'Committing...';
		case 'pushing':
			return 'Pushing...';
		default:
			return 'Committing...';
	}
}
