export type ViewportSnapshot = {
	followOutput: boolean;
	linesFromBottom: number;
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
		return { followOutput: true, linesFromBottom: 0 };
	}
	return { followOutput: false, linesFromBottom: baseY - viewportY };
};

export const resolveViewportTargetLine = (
	snapshot: ViewportSnapshot,
	nextBaseY: number,
): number | null => {
	const baseY = clampNonNegativeInt(nextBaseY);
	if (snapshot.followOutput || snapshot.linesFromBottom <= 0) {
		return null;
	}
	const linesFromBottom = Math.min(clampNonNegativeInt(snapshot.linesFromBottom), baseY);
	return Math.max(0, baseY - linesFromBottom);
};
