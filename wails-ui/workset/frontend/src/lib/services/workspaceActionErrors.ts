export const formatWorkspaceActionError = (err: unknown, fallback: string): string => {
	if (err instanceof Error) return err.message;
	if (typeof err === 'string') return err;
	if (err && typeof err === 'object' && 'message' in err) {
		const message = (err as { message?: string }).message;
		if (typeof message === 'string') return message;
	}
	return fallback;
};
