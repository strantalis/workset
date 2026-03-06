export const shouldClearPreviousWorkspaceTerminalActivity = ({
	previousWorkspaceId,
	nextWorkspaceId,
	previousWorkspacePoppedOut,
}: {
	previousWorkspaceId: string | null | undefined;
	nextWorkspaceId: string;
	previousWorkspacePoppedOut: boolean;
}): boolean =>
	Boolean(
		previousWorkspaceId && previousWorkspaceId !== nextWorkspaceId && !previousWorkspacePoppedOut,
	);
