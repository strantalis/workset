import type { TerminalLayout, TerminalLayoutPayload } from '../types';
import type {
	TerminalLayout as BoundTerminalLayout,
	TerminalLayoutRequest as BoundTerminalLayoutRequest,
} from '../../../bindings/workset/models';
import {
	CreateWorkspaceTerminal,
	GetTerminalBacklog,
	GetTerminalBootstrap,
	GetTerminalSnapshot,
	GetWorkspaceTerminalLayout,
	GetWorkspaceTerminalStatus,
	LogTerminalDebug,
	SetWorkspaceTerminalLayout,
	StopWorkspaceTerminalForWindowName,
	WriteWorkspaceTerminalForWindowName,
} from '../../../wailsjs/go/main/App';
import { getCurrentWindowName } from '../windowContext';

export type TerminalBacklogResponse = {
	workspaceId: string;
	terminalId: string;
	data: string;
	nextOffset: number;
	truncated: boolean;
	source?: string;
};

export type TerminalSnapshotResponse = {
	workspaceId: string;
	terminalId: string;
	data: string;
	source?: string;
	kitty?: {
		images?: Array<{
			id: string;
			format?: string;
			width?: number;
			height?: number;
			data?: string | number[];
		}>;
		placements?: Array<{
			id: number;
			imageId: string;
			row: number;
			col: number;
			rows: number;
			cols: number;
			x?: number;
			y?: number;
			z?: number;
		}>;
	};
};

export type TerminalBootstrapResponse = {
	workspaceId: string;
	terminalId: string;
	snapshot?: string;
	snapshotSource?: string;
	kitty?: {
		images?: Array<{
			id: string;
			format?: string;
			width?: number;
			height?: number;
			data?: string | number[];
		}>;
		placements?: Array<{
			id: number;
			imageId: string;
			row: number;
			col: number;
			rows: number;
			cols: number;
			x?: number;
			y?: number;
			z?: number;
		}>;
	};
	backlog?: string;
	backlogSource?: string;
	backlogTruncated?: boolean;
	nextOffset?: number;
	source?: string;
	altScreen?: boolean;
	mouse?: boolean;
	mouseSGR?: boolean;
	mouseEncoding?: string;
	safeToReplay?: boolean;
	initialCredit?: number;
};

export type WorkspaceTerminalStatusResponse = {
	workspaceId: string;
	terminalId?: string;
	active: boolean;
	error?: string;
};

export async function fetchWorkspaceTerminalStatus(
	workspaceId: string,
	terminalId: string,
): Promise<WorkspaceTerminalStatusResponse> {
	return GetWorkspaceTerminalStatus(workspaceId, terminalId);
}

export async function fetchTerminalSnapshot(
	workspaceId: string,
	terminalId: string,
): Promise<TerminalSnapshotResponse> {
	const snapshot = await GetTerminalSnapshot(workspaceId, terminalId);
	return {
		...snapshot,
		kitty: snapshot.kitty ?? undefined,
	};
}

export async function fetchTerminalBootstrap(
	workspaceId: string,
	terminalId: string,
): Promise<TerminalBootstrapResponse> {
	const payload = await GetTerminalBootstrap(workspaceId, terminalId);
	return {
		...payload,
		kitty: payload.kitty ?? undefined,
	};
}

export async function logTerminalDebug(
	workspaceId: string,
	terminalId: string,
	event: string,
	details = '',
): Promise<void> {
	await LogTerminalDebug({ workspaceId, terminalId, event, details });
}

export async function fetchTerminalBacklog(
	workspaceId: string,
	terminalId: string,
	since: number,
): Promise<TerminalBacklogResponse> {
	return GetTerminalBacklog(workspaceId, terminalId, since);
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

export async function writeWorkspaceTerminal(
	workspaceId: string,
	terminalId: string,
	data: string,
): Promise<void> {
	const windowName = await getCurrentWindowName();
	await WriteWorkspaceTerminalForWindowName(workspaceId, terminalId, data, windowName);
}
