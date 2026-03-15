export type TerminalPerformanceSnapshot = {
	fps: number;
	frameTimeMs: number;
};

type TerminalPerformanceSamplerOptions = {
	sampleWindowMs?: number;
};

const DEFAULT_SAMPLE_WINDOW_MS = 500;

export const createTerminalPerformanceSampler = (
	options: TerminalPerformanceSamplerOptions = {},
) => {
	const sampleWindowMs = options.sampleWindowMs ?? DEFAULT_SAMPLE_WINDOW_MS;
	let sampleStartedAt: number | null = null;
	let frameCount = 0;
	let lastFrameAt = 0;
	let fps = 0;
	let frameTimeMs = 0;

	const sampleFrame = (timestamp: number): TerminalPerformanceSnapshot => {
		if (sampleStartedAt === null) {
			sampleStartedAt = timestamp;
			lastFrameAt = timestamp;
			frameCount = 1;
			return { fps, frameTimeMs };
		}

		const delta = Math.max(0, timestamp - lastFrameAt);
		lastFrameAt = timestamp;
		frameCount += 1;
		frameTimeMs = frameTimeMs === 0 ? delta : frameTimeMs * 0.8 + delta * 0.2;

		const elapsed = timestamp - sampleStartedAt;
		if (elapsed >= sampleWindowMs) {
			fps = (frameCount * 1000) / elapsed;
			frameCount = 0;
			sampleStartedAt = timestamp;
		}

		return { fps, frameTimeMs };
	};

	const reset = (): void => {
		sampleStartedAt = null;
		frameCount = 0;
		lastFrameAt = 0;
		fps = 0;
		frameTimeMs = 0;
	};

	return {
		sampleFrame,
		reset,
		getSnapshot: (): TerminalPerformanceSnapshot => ({
			fps,
			frameTimeMs,
		}),
	};
};
