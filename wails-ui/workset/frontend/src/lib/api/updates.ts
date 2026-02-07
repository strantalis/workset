import type {
	AppVersion,
	UpdateCheckResult,
	UpdatePreferences,
	UpdateStartResult,
	UpdateState,
} from '../types';
import {
	CheckForUpdates,
	GetAppVersion,
	GetUpdatePreferences,
	GetUpdateState,
	SetUpdatePreferences,
	StartUpdate,
} from '../../../wailsjs/go/main/App';

export async function fetchAppVersion(): Promise<AppVersion> {
	return (await GetAppVersion()) as AppVersion;
}

export async function fetchUpdatePreferences(): Promise<UpdatePreferences> {
	return (await GetUpdatePreferences()) as UpdatePreferences;
}

export async function setUpdatePreferences(
	input: Partial<UpdatePreferences> & { channel?: string },
): Promise<UpdatePreferences> {
	const payload: { channel: string; autoCheck?: boolean } = {
		channel: input.channel ?? '',
	};
	if (input.autoCheck !== undefined) {
		payload.autoCheck = input.autoCheck;
	}
	return (await SetUpdatePreferences(payload)) as UpdatePreferences;
}

export async function checkForUpdates(channel?: string): Promise<UpdateCheckResult> {
	return (await CheckForUpdates({ channel: channel ?? '' })) as UpdateCheckResult;
}

export async function startAppUpdate(channel?: string): Promise<UpdateStartResult> {
	return (await StartUpdate({ channel: channel ?? '' })) as UpdateStartResult;
}

export async function fetchUpdateState(): Promise<UpdateState> {
	return (await GetUpdateState()) as UpdateState;
}
