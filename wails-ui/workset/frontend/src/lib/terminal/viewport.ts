export type ViewportSnapshot = {
	followOutput: boolean;
	viewportLine: number;
};

type ViewportPosition = {
	baseY: number;
	viewportY: number;
};

const clampNonNegativeInt = (value: number): number => {
	if (!Number.isFinite(value)) return 0;
	return Math.max(0, Math.trunc(value));
};

export const captureViewportSnapshot = (position: ViewportPosition): ViewportSnapshot => {
	const baseY = clampNonNegativeInt(position.baseY);
	const viewportY = clampNonNegativeInt(position.viewportY);
	if (viewportY >= baseY) {
		return { followOutput: true, viewportLine: baseY };
	}
	return { followOutput: false, viewportLine: viewportY };
};

export const resolveViewportTargetLine = (
	snapshot: ViewportSnapshot,
	nextBaseY: number,
): number | null => {
	const baseY = clampNonNegativeInt(nextBaseY);
	if (snapshot.followOutput) {
		return null;
	}
	return Math.min(clampNonNegativeInt(snapshot.viewportLine), baseY);
};
