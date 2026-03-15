import ghosttyWasmUrl from 'ghostty-web/ghostty-vt.wasm?url';
import { init } from 'ghostty-web';

let initialized = false;
let initPromise: Promise<void> | null = null;

export const ensureGhosttyInitialized = async (): Promise<void> => {
	if (initialized) return;
	if (!initPromise) {
		initPromise = init(ghosttyWasmUrl).then(
			() => {
				initialized = true;
			},
			() => {
				initialized = false;
				initPromise = null;
				throw new Error('ghostty-web initialization failed');
			},
		);
	}
	await initPromise;
};
