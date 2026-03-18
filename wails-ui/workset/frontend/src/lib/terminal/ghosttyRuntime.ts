import { init } from '@strantalis/workset-ghostty-web';

let initialized = false;
let initPromise: Promise<void> | null = null;

export const ensureGhosttyInitialized = async (): Promise<void> => {
	if (initialized) return;
	if (!initPromise) {
		initPromise = init().then(
			() => {
				initialized = true;
			},
			() => {
				initialized = false;
				initPromise = null;
				throw new Error('@strantalis/workset-ghostty-web initialization failed');
			},
		);
	}
	await initPromise;
};
