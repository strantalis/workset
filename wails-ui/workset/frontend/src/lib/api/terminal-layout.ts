import type { TerminalLayout, TerminalLayoutPayload } from '../types';
import type {
	TerminalLayout as BoundTerminalLayout,
	TerminalLayoutRequest as BoundTerminalLayoutRequest,
} from '../../../bindings/workset/models';
import {
	CreateWorkspaceTerminal,
	GetWorkspaceTerminalLayout,
	LogTerminalDebug,
	SetWorkspaceTerminalLayout,
	StopWorkspaceTerminalForWindowName,
} from '../../../bindings/workset/app';
import { getCurrentWindowName } from '../windowContext';

export async function logTerminalDebug(
	workspaceId: string,
	terminalId: string,
	event: string,
	details = '',
): Promise<void> {
	await LogTerminalDebug({ workspaceId, terminalId, event, details });
}

export async function createWorkspaceTerminal(
	workspaceId: string,
): Promise<{ workspaceId: string; terminalId: string }> {
	return CreateWorkspaceTerminal(workspaceId);
}

export async function stopWorkspaceTerminal(
	workspaceId: string,
	terminalId: string,
): Promise<void> {
	const windowName = await getCurrentWindowName();
	await StopWorkspaceTerminalForWindowName(workspaceId, terminalId, windowName);
}

export async function fetchWorkspaceTerminalLayout(
	workspaceId: string,
): Promise<TerminalLayoutPayload> {
	return (await GetWorkspaceTerminalLayout(workspaceId)) as TerminalLayoutPayload;
}

export async function persistWorkspaceTerminalLayout(
	workspaceId: string,
	layout: TerminalLayout,
): Promise<void> {
	await SetWorkspaceTerminalLayout({
		workspaceId,
		layout: layout as unknown as BoundTerminalLayout,
	} as unknown as BoundTerminalLayoutRequest);
}
