import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	createKittyState,
	createTerminalKittyController,
	type TerminalKittyHandle,
} from './terminalKittyImageController';

type TestHandle = TerminalKittyHandle;

const flushPromises = async (): Promise<void> => {
	await Promise.resolve();
	await Promise.resolve();
};

const makeContainer = (): HTMLDivElement => {
	const container = document.createElement('div') as HTMLDivElement;
	Object.defineProperty(container, 'getBoundingClientRect', {
		value: () =>
			({
				x: 0,
				y: 0,
				top: 0,
				left: 0,
				right: 240,
				bottom: 120,
				width: 240,
				height: 120,
				toJSON: () => ({}),
			}) as DOMRect,
	});
	return container;
};

afterEach(() => {
	vi.restoreAllMocks();
});

describe('terminalKittyImageController', () => {
	it('creates overlay canvases and coalesces render scheduling', () => {
		const clearRect = vi.fn();
		const drawImage = vi.fn();
		vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
			clearRect,
			drawImage,
		} as unknown as CanvasRenderingContext2D);

		const requestFrame = vi.fn((_callback: (timestamp: number) => void) => 1);

		const handle: TestHandle = {
			terminal: { cols: 120, rows: 30 },
			container: makeContainer(),
			kittyState: createKittyState(),
		};
		const controller = createTerminalKittyController<TestHandle>({
			getHandle: () => handle,
			requestFrame,
			createBitmap: vi.fn(async () => ({}) as ImageBitmap),
			getDevicePixelRatio: () => 2,
		});

		controller.ensureOverlay('ws::term');
		controller.ensureOverlay('ws::term');

		expect(handle.container.querySelectorAll('canvas')).toHaveLength(2);
		expect(requestFrame).toHaveBeenCalledTimes(1);
		expect(handle.kittyOverlay?.underlay.className).toBe('kitty-underlay');
		expect(handle.kittyOverlay?.overlay.className).toBe('kitty-overlay');

		const frameCallback = requestFrame.mock.calls[0]?.[0] as
			| ((timestamp: number) => void)
			| undefined;
		frameCallback?.(0);
		expect(clearRect).toHaveBeenCalled();
	});

	it('hydrates snapshot data and decodes image bitmaps', async () => {
		vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
			clearRect: vi.fn(),
			drawImage: vi.fn(),
		} as unknown as CanvasRenderingContext2D);
		const createBitmap = vi.fn(async () => ({}) as ImageBitmap);

		const handle: TestHandle = {
			terminal: { cols: 80, rows: 24 },
			container: makeContainer(),
			kittyState: createKittyState(),
		};
		const controller = createTerminalKittyController<TestHandle>({
			getHandle: () => handle,
			requestFrame: vi.fn(() => 1),
			createBitmap,
			getDevicePixelRatio: () => 1,
		});

		await controller.applyEvent('ws::term', {
			kind: 'snapshot',
			snapshot: {
				images: [{ id: 'img-1', data: btoa('payload') }],
				placements: [{ id: 7, imageId: 'img-1', row: 1, col: 2, rows: 3, cols: 4 }],
			},
		});
		await flushPromises();

		expect(handle.kittyState?.images.get('img-1')?.data.length).toBeGreaterThan(0);
		expect(handle.kittyState?.placements.get('7')).toEqual(
			expect.objectContaining({ id: 7, imageId: 'img-1', row: 1, col: 2, rows: 3, cols: 4 }),
		);
		expect(createBitmap).toHaveBeenCalledTimes(1);
		expect(handle.kittyState?.images.get('img-1')?.bitmap).toBeDefined();
	});

	it('supports image/placement deletes and clear events', async () => {
		const clearRect = vi.fn();
		vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
			clearRect,
			drawImage: vi.fn(),
		} as unknown as CanvasRenderingContext2D);
		const handle: TestHandle = {
			terminal: { cols: 80, rows: 24 },
			container: makeContainer(),
			kittyState: createKittyState(),
		};
		const controller = createTerminalKittyController<TestHandle>({
			getHandle: () => handle,
			requestFrame: vi.fn(() => 1),
			createBitmap: vi.fn(async () => ({}) as ImageBitmap),
			getDevicePixelRatio: () => 1,
		});

		controller.ensureOverlay('ws::term');
		await controller.applyEvent('ws::term', {
			kind: 'image',
			image: { id: 'img-1', data: [1, 2, 3] },
		});
		await controller.applyEvent('ws::term', {
			kind: 'placement',
			placement: { id: 99, imageId: 'img-1', row: 1, col: 1, rows: 1, cols: 1 },
		});
		await controller.applyEvent('ws::term', {
			kind: 'delete',
			delete: { imageId: 'img-1', placementId: 99 },
		});

		expect(handle.kittyState?.images.size).toBe(0);
		expect(handle.kittyState?.placements.size).toBe(0);

		await controller.applyEvent('ws::term', { kind: 'clear' });

		expect(clearRect).toHaveBeenCalled();
		expect(handle.kittyState?.images.size).toBe(0);
		expect(handle.kittyState?.placements.size).toBe(0);
	});
});
