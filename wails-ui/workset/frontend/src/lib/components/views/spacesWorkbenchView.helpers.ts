export type SurfaceTab = 'terminal' | 'pull-requests';

export type WorkbenchLayoutMode = 'terminal' | 'terminal-with-prs';

export const resolveWorkbenchLayout = (activeSurface: SurfaceTab): WorkbenchLayoutMode => {
	if (activeSurface === 'pull-requests') return 'terminal-with-prs';
	return 'terminal';
};
