import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	createTerminalAttachState,
	createTerminalRendererAddonState,
	type AttachRendererHandle,
} from './terminalAttachRendererState';

type RendererAddonLoader = NonNullable<
	Parameters<typeof createTerminalRendererAddonState>[0]['loadRendererAddon']
>;
type RendererAddonLoaderInput = Parameters<RendererAddonLoader>[0];

describe('terminalAttachRendererState', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('schedules terminal disposal on detach and cancels it on reattach', () => {
		const onDispose = vi.fn();
		const attachState = createTerminalAttachState({
			disposeAfterMs: 1000,
			onDispose,
			setTimeoutFn: (callback, timeoutMs) => setTimeout(callback, timeoutMs),
			clearTimeoutFn: (handle) => clearTimeout(handle),
		});

		attachState.markAttached('ws::term');
		attachState.markDetached('ws::term');

		vi.advanceTimersByTime(900);
		expect(onDispose).not.toHaveBeenCalled();

		attachState.markAttached('ws::term');
		vi.advanceTimersByTime(200);
		expect(onDispose).not.toHaveBeenCalled();

		attachState.markDetached('ws::term');
		vi.advanceTimersByTime(1000);
		expect(onDispose).toHaveBeenCalledTimes(1);
		expect(onDispose).toHaveBeenCalledWith('ws::term');
	});

	it('bridges renderer addon callbacks through terminal id', async () => {
		const setRendererMode = vi.fn();
		const setRenderer = vi.fn();
		const onRendererUnavailable = vi.fn();
		const onComplete = vi.fn();
		const addon = { dispose: vi.fn() } as unknown as NonNullable<
			AttachRendererHandle['webglAddon']
		>;
		const loadRendererAddon = vi.fn(async (input: RendererAddonLoaderInput) => {
			input.setRendererMode('webgl');
			input.setRenderer('webgl');
			input.onComplete();
			return addon;
		});
		const rendererState = createTerminalRendererAddonState({
			setRendererMode,
			setRenderer,
			onRendererUnavailable,
			onComplete,
			loadRendererAddon: loadRendererAddon as unknown as RendererAddonLoader,
		});
		const handle = {
			terminal: {} as AttachRendererHandle['terminal'],
		} as AttachRendererHandle;

		await rendererState.load('ws::term', handle);

		expect(loadRendererAddon).toHaveBeenCalledTimes(1);
		expect(setRendererMode).toHaveBeenCalledWith('ws::term', 'webgl');
		expect(setRenderer).toHaveBeenCalledWith('ws::term', 'webgl');
		expect(onComplete).toHaveBeenCalledWith('ws::term');
		expect(onRendererUnavailable).not.toHaveBeenCalled();
		expect(handle.webglAddon).toBe(addon);
	});

	it('propagates renderer unavailable callback and keeps addon undefined on failure', async () => {
		const error = new Error('gpu unavailable');
		const onRendererUnavailable = vi.fn();
		const loadRendererAddon = vi.fn(async (input: RendererAddonLoaderInput) => {
			input.setRendererMode('webgl');
			input.setRenderer('unknown');
			input.onRendererUnavailable(error);
			input.onComplete();
			return undefined;
		});
		const rendererState = createTerminalRendererAddonState({
			setRendererMode: vi.fn(),
			setRenderer: vi.fn(),
			onRendererUnavailable,
			onComplete: vi.fn(),
			loadRendererAddon: loadRendererAddon as unknown as RendererAddonLoader,
		});
		const handle = {
			terminal: {} as AttachRendererHandle['terminal'],
			webglAddon: { dispose: vi.fn() } as unknown as AttachRendererHandle['webglAddon'],
		} as AttachRendererHandle;

		await rendererState.load('ws::term', handle);

		expect(onRendererUnavailable).toHaveBeenCalledTimes(1);
		expect(onRendererUnavailable).toHaveBeenCalledWith('ws::term', error);
		expect(handle.webglAddon).toBeUndefined();
	});
});
