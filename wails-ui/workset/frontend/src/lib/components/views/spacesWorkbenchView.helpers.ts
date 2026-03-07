export type SurfaceTab = 'terminal' | 'pull-requests';

export type WorkbenchLayoutMode = 'terminal' | 'terminal-with-document' | 'terminal-with-prs';

export const resolveWorkbenchLayout = (
	activeSurface: SurfaceTab,
	hasDocumentSession: boolean,
): WorkbenchLayoutMode => {
	if (activeSurface === 'pull-requests') return 'terminal-with-prs';
	if (hasDocumentSession) return 'terminal-with-document';
	return 'terminal';
};
