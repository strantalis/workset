import { describe, expect, test } from 'vitest';
import { captureViewportSnapshot, resolveViewportTargetLine } from './viewport';

describe('viewport snapshot', () => {
	test('follows output when viewport is at bottom', () => {
		const snapshot = captureViewportSnapshot({ baseY: 120, viewportY: 120 });
		const nextLine = resolveViewportTargetLine(snapshot, 260);
		expect(snapshot).toEqual({ followOutput: true, linesFromBottom: 0 });
		expect(nextLine).toBeNull();
	});

	test('preserves distance from bottom when user scrolled up', () => {
		const snapshot = captureViewportSnapshot({ baseY: 120, viewportY: 90 });
		const nextLine = resolveViewportTargetLine(snapshot, 260);
		expect(snapshot).toEqual({ followOutput: false, linesFromBottom: 30 });
		expect(nextLine).toBe(230);
	});

	test('clamps target line to top when new buffer is shorter', () => {
		const snapshot = captureViewportSnapshot({ baseY: 100, viewportY: 20 });
		const nextLine = resolveViewportTargetLine(snapshot, 40);
		expect(nextLine).toBe(0);
	});

	test('treats out-of-range viewport as follow mode', () => {
		const snapshot = captureViewportSnapshot({ baseY: 75, viewportY: 100 });
		const nextLine = resolveViewportTargetLine(snapshot, 160);
		expect(snapshot).toEqual({ followOutput: true, linesFromBottom: 0 });
		expect(nextLine).toBeNull();
	});
});
