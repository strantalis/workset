import { toErrorMessage as toErrorMessageApi } from '../errors';
import type { UpdateCheckResult, UpdatePreferences, UpdateState } from '../types';
import {
	checkForUpdates as checkForUpdatesApi,
	fetchUpdatePreferences as fetchUpdatePreferencesApi,
	fetchUpdateState as fetchUpdateStateApi,
	setUpdatePreferences as setUpdatePreferencesApi,
	startAppUpdate as startAppUpdateApi,
} from '../api/updates';
import {
	DEFAULT_UPDATE_PREFERENCES,
	UPDATE_RECHECK_INTERVAL_MS,
	resolveUpdateNotesUrl,
} from '../updatePreferences';

export type UpdateNotificationCardModel = {
	mode: 'available' | 'applying';
	latestVersion: string;
	message: string;
	notesUrl: string;
	error: string | null;
};

export type UpdateNotificationController = {
	readonly busy: boolean;
	readonly updatePreferences: UpdatePreferences;
	readonly card: UpdateNotificationCardModel | null;
	init: () => Promise<void>;
	applyPreferences: (preferences: UpdatePreferences) => Promise<void>;
	dismiss: () => Promise<void>;
	startUpdate: () => Promise<void>;
	destroy: () => void;
};

type UpdateNotificationDeps = {
	fetchUpdatePreferences: () => Promise<UpdatePreferences>;
	fetchUpdateState: () => Promise<UpdateState>;
	checkForUpdates: (channel?: string) => Promise<UpdateCheckResult>;
	setUpdatePreferences: (
		input: Partial<UpdatePreferences> & { channel?: string },
	) => Promise<UpdatePreferences>;
	startAppUpdate: (channel?: string) => Promise<{ state: UpdateState }>;
	setInterval: (handler: () => void, timeoutMs: number) => ReturnType<typeof setInterval>;
	clearInterval: (timer: ReturnType<typeof setInterval>) => void;
	toErrorMessage: (error: unknown, fallback: string) => string;
};

const normalizePreferences = (preferences: UpdatePreferences): UpdatePreferences => ({
	channel: preferences.channel === 'alpha' ? 'alpha' : 'stable',
	autoCheck: preferences.autoCheck,
	dismissedVersion: preferences.dismissedVersion ?? '',
});

export function createUpdateNotificationController(
	overrides: Partial<UpdateNotificationDeps> = {},
): UpdateNotificationController {
	const deps: UpdateNotificationDeps = {
		fetchUpdatePreferences: fetchUpdatePreferencesApi,
		fetchUpdateState: fetchUpdateStateApi,
		checkForUpdates: checkForUpdatesApi,
		setUpdatePreferences: setUpdatePreferencesApi,
		startAppUpdate: startAppUpdateApi,
		setInterval: (handler, timeoutMs) => window.setInterval(handler, timeoutMs),
		clearInterval: (timer) => window.clearInterval(timer),
		toErrorMessage: toErrorMessageApi,
		...overrides,
	};

	let updatePreferences = $state<UpdatePreferences>(DEFAULT_UPDATE_PREFERENCES);
	let updateCheck = $state<UpdateCheckResult | null>(null);
	let updateState = $state<UpdateState | null>(null);
	let busy = $state(false);
	let actionError = $state<string | null>(null);
	let intervalHandle = $state<ReturnType<typeof setInterval> | null>(null);
	let checking = false;

	const syncUpdateState = async (): Promise<void> => {
		updateState = await deps.fetchUpdateState().catch(() => updateState);
	};

	const scheduleRecheck = (): void => {
		if (intervalHandle) {
			deps.clearInterval(intervalHandle);
			intervalHandle = null;
		}
		if (!updatePreferences.autoCheck) {
			return;
		}
		intervalHandle = deps.setInterval(() => {
			void runAutoCheck();
		}, UPDATE_RECHECK_INTERVAL_MS);
	};

	const runAutoCheck = async (): Promise<void> => {
		if (!updatePreferences.autoCheck || checking || busy) {
			return;
		}
		checking = true;
		try {
			updateCheck = await deps.checkForUpdates(updatePreferences.channel);
			await syncUpdateState();
		} catch {
			// Auto-check failures should stay quiet; the manual Settings flow exposes errors.
		} finally {
			checking = false;
		}
	};

	const applyPreferences = async (preferences: UpdatePreferences): Promise<void> => {
		const previous = updatePreferences;
		updatePreferences = normalizePreferences(preferences);
		scheduleRecheck();
		const shouldCheckNow =
			updatePreferences.autoCheck &&
			(!previous.autoCheck || previous.channel !== updatePreferences.channel);
		if (shouldCheckNow) {
			await runAutoCheck();
		}
	};

	const dismiss = async (): Promise<void> => {
		if (busy || updateCheck?.status !== 'update_available') {
			return;
		}
		busy = true;
		actionError = null;
		try {
			updatePreferences = normalizePreferences(
				await deps.setUpdatePreferences({ dismissedVersion: updateCheck.latestVersion }),
			);
			scheduleRecheck();
		} catch (error) {
			actionError = deps.toErrorMessage(error, 'Failed to dismiss update notification.');
		} finally {
			busy = false;
		}
	};

	const startUpdate = async (): Promise<void> => {
		if (busy) {
			return;
		}
		busy = true;
		actionError = null;
		try {
			const result = await deps.startAppUpdate(updatePreferences.channel);
			updateState = result.state;
		} catch (error) {
			actionError = deps.toErrorMessage(error, 'Failed to start update.');
		} finally {
			busy = false;
		}
	};

	const init = async (): Promise<void> => {
		updatePreferences = normalizePreferences(
			await deps.fetchUpdatePreferences().catch(() => DEFAULT_UPDATE_PREFERENCES),
		);
		updateState = await deps.fetchUpdateState().catch((): UpdateState | null => null);
		scheduleRecheck();
		if (updatePreferences.autoCheck) {
			await runAutoCheck();
		}
	};

	const destroy = (): void => {
		if (intervalHandle) {
			deps.clearInterval(intervalHandle);
			intervalHandle = null;
		}
	};

	return {
		get busy() {
			return busy;
		},
		get updatePreferences() {
			return updatePreferences;
		},
		get card() {
			if (updateState?.phase === 'applying') {
				return {
					mode: 'applying' as const,
					latestVersion: updateState.latestVersion || updateCheck?.latestVersion || '',
					message: updateState.message || 'Applying update...',
					notesUrl: resolveUpdateNotesUrl(updateCheck?.release),
					error: null,
				};
			}
			if (
				updateCheck?.status !== 'update_available' ||
				!updatePreferences.autoCheck ||
				updateCheck.latestVersion === updatePreferences.dismissedVersion
			) {
				return null;
			}
			return {
				mode: 'available' as const,
				latestVersion: updateCheck.latestVersion,
				message: updateCheck.message,
				notesUrl: resolveUpdateNotesUrl(updateCheck.release),
				error:
					actionError ??
					(updateState?.phase === 'failed' ? updateState.error || updateState.message : null),
			};
		},
		init,
		applyPreferences,
		dismiss,
		startUpdate,
		destroy,
	};
}
