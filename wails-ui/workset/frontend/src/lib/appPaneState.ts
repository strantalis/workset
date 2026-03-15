export type WorkbenchSurface = 'terminal' | 'pull-requests';
export type WorkbenchPaneIntent = 'pull-requests' | 'files';

export const resolveWorkbenchPaneState = (input: {
	surface: WorkbenchSurface;
	filesOpen: boolean;
	intent: WorkbenchPaneIntent;
}): { surface: WorkbenchSurface; filesOpen: boolean } => {
	if (input.intent === 'pull-requests') {
		return {
			surface: input.surface === 'pull-requests' ? 'terminal' : 'pull-requests',
			filesOpen: false,
		};
	}

	return {
		surface: 'terminal',
		filesOpen: !input.filesOpen,
	};
};
