const normalizeMessage = (value: string): string | null => {
	const trimmed = value.trim();
	return trimmed.length > 0 ? trimmed : null;
};

const parseStructuredErrorMessage = (value: string): string | null => {
	const trimmed = value.trim();
	if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) {
		return null;
	}
	try {
		const parsed = JSON.parse(trimmed);
		return extractErrorMessage(parsed);
	} catch {
		return null;
	}
};

const extractErrorMessage = (value: unknown): string | null => {
	if (typeof value === 'string') {
		const normalized = normalizeMessage(value);
		if (!normalized) return null;
		const structured = parseStructuredErrorMessage(normalized);
		return structured ?? normalized;
	}
	if (!value || typeof value !== 'object') {
		return null;
	}
	if ('message' in value) {
		return extractErrorMessage((value as { message?: unknown }).message);
	}
	return null;
};

export const formatWorkspaceActionError = (err: unknown, fallback: string): string =>
	extractErrorMessage(err) ?? fallback;
