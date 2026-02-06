import { describe, expect, test } from 'vitest';
import { captureViewportSnapshot, resolveViewportTargetLine } from './viewport';

describe('viewport snapshot', () => {
	test('follows output when viewport is at bottom', () => {
		const snapshot = captureViewportSnapshot({ baseY: 120, viewportY: 120 });
		const nextLine = resolveViewportTargetLine(snapshot, 260);
		expect(snapshot).toEqual({ followOutput: true, viewportLine: 120 });
		expect(nextLine).toBeNull();
	});

	test('preserves absolute viewport line when user scrolled up', () => {
		const snapshot = captureViewportSnapshot({ baseY: 120, viewportY: 90 });
		const nextLine = resolveViewportTargetLine(snapshot, 260);
		expect(snapshot).toEqual({ followOutput: false, viewportLine: 90 });
		expect(nextLine).toBe(90);
	});

	test('clamps target line to base when new buffer is shorter', () => {
		const snapshot = captureViewportSnapshot({ baseY: 100, viewportY: 90 });
		const nextLine = resolveViewportTargetLine(snapshot, 40);
		expect(nextLine).toBe(40);
	});

	test('treats out-of-range viewport as follow mode', () => {
		const snapshot = captureViewportSnapshot({ baseY: 75, viewportY: 100 });
		const nextLine = resolveViewportTargetLine(snapshot, 160);
		expect(snapshot).toEqual({ followOutput: true, viewportLine: 75 });
		expect(nextLine).toBeNull();
	});
});
