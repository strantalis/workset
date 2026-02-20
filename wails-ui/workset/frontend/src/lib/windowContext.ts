import { GetCurrentWindowName } from '../../bindings/workset/app';

const query = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;

const requestedWindowName = query?.get('window')?.trim() ?? '';
const fallbackWindowName = requestedWindowName !== '' ? requestedWindowName : 'main';
let resolvedWindowName: string | null = null;
let resolvingWindowName: Promise<string> | null = null;

const normalizeWindowName = (value: string | null | undefined): string => {
	const candidate = value?.trim();
	if (!candidate) return fallbackWindowName;
	return candidate;
};

export const getCurrentWindowNameHint = (): string =>
	normalizeWindowName(resolvedWindowName ?? requestedWindowName);

export const getCurrentWindowName = async (): Promise<string> => {
	if (resolvedWindowName) return resolvedWindowName;
	if (resolvingWindowName) return resolvingWindowName;
	resolvingWindowName = (async () => {
		try {
			const backendWindow = await GetCurrentWindowName();
			const normalized = normalizeWindowName(backendWindow);
			resolvedWindowName = normalized;
			return normalized;
		} catch {
			const normalized = normalizeWindowName('');
			resolvedWindowName = normalized;
			return normalized;
		} finally {
			resolvingWindowName = null;
		}
	})();
	return resolvingWindowName;
};
