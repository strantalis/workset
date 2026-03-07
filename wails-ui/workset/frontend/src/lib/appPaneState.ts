export type CockpitSurface = 'terminal' | 'pull-requests';
export type CockpitPaneIntent = 'pull-requests' | 'files';

export const resolveCockpitPaneState = (input: {
	surface: CockpitSurface;
	filesOpen: boolean;
	intent: CockpitPaneIntent;
}): { surface: CockpitSurface; filesOpen: boolean } => {
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
