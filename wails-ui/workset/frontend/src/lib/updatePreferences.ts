import type { UpdatePreferences, UpdateRelease } from './types';

export const DEFAULT_UPDATE_PREFERENCES: UpdatePreferences = {
	channel: 'stable',
	autoCheck: true,
	dismissedVersion: '',
};

export const UPDATE_PREFERENCES_CHANGED_EVENT = 'workset:update-preferences-changed';
export const UPDATE_RECHECK_INTERVAL_MS = 6 * 60 * 60 * 1000;
export const DEFAULT_UPDATE_RELEASES_URL = 'https://github.com/anomalyco/workset/releases';

export type UpdatePreferencesChangedDetail = {
	preferences: UpdatePreferences;
};

export function resolveUpdateNotesUrl(release?: Pick<UpdateRelease, 'notesUrl'> | null): string {
	const notesUrl = release?.notesUrl?.trim() ?? '';
	return notesUrl !== '' ? notesUrl : DEFAULT_UPDATE_RELEASES_URL;
}

export function dispatchUpdatePreferencesChanged(preferences: UpdatePreferences): void {
	if (typeof window === 'undefined' || typeof CustomEvent === 'undefined') {
		return;
	}
	window.dispatchEvent(
		new CustomEvent<UpdatePreferencesChangedDetail>(UPDATE_PREFERENCES_CHANGED_EVENT, {
			detail: { preferences },
		}),
	);
}
