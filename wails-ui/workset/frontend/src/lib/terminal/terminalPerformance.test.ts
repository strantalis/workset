import { describe, expect, it } from 'vitest';
import { createTerminalPerformanceSampler } from './terminalPerformance';

describe('terminalPerformance', () => {
	it('computes fps after the sample window elapses', () => {
		const sampler = createTerminalPerformanceSampler({ sampleWindowMs: 500 });

		sampler.sampleFrame(0);
		sampler.sampleFrame(16);
		sampler.sampleFrame(32);
		sampler.sampleFrame(48);
		const snapshot = sampler.sampleFrame(500);

		expect(snapshot.fps).toBeGreaterThan(9);
		expect(snapshot.fps).toBeLessThan(11);
		expect(snapshot.frameTimeMs).toBeGreaterThan(0);
	});

	it('resets accumulated values', () => {
		const sampler = createTerminalPerformanceSampler({ sampleWindowMs: 100 });

		sampler.sampleFrame(0);
		sampler.sampleFrame(16);
		sampler.sampleFrame(120);
		expect(sampler.getSnapshot().fps).toBeGreaterThan(0);

		sampler.reset();

		expect(sampler.getSnapshot()).toEqual({
			fps: 0,
			frameTimeMs: 0,
		});
	});
});
