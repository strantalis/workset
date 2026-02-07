import { describe, expect, it, vi } from 'vitest';
import {
	createTerminalAttachOpenLifecycle,
	type TerminalAttachOpenHandle,
} from './terminalAttachOpenLifecycle';

const createHandle = (): TerminalAttachOpenHandle => {
	const host = document.createElement('div');
	const terminal: TerminalAttachOpenHandle['terminal'] = {
		element: null,
		open: vi.fn((container: HTMLElement) => {
			terminal.element = { parentElement: container };
		}),
		focus: vi.fn(),
	};
	return {
		terminal,
		container: host,
	};
};

const flushPromises = async (): Promise<void> => {
	await Promise.resolve();
	await Promise.resolve();
};

describe('terminalAttachOpenLifecycle', () => {
	it('opens terminal, loads renderer addon, and re-fits after fonts are ready', async () => {
		const container = document.createElement('div') as HTMLDivElement;
		const handle = createHandle();
		let resolveFonts: (() => void) | undefined;
		const fontsReady = new Promise<void>((resolve) => {
			resolveFonts = resolve;
		});
		const ensureOverlay = vi.fn();
		const loadRendererAddon = vi.fn();
		const fitWithPreservedViewport = vi.fn();
		const resizeToFit = vi.fn();
		const scheduleFitStabilization = vi.fn();
		const flushOutput = vi.fn();
		const markAttached = vi.fn();
		const lifecycle = createTerminalAttachOpenLifecycle({
			getHandle: () => handle,
			ensureOverlay,
			loadRendererAddon,
			fitWithPreservedViewport,
			resizeToFit,
			scheduleFitStabilization,
			flushOutput,
			markAttached,
			resolveFontsReady: () => fontsReady,
		});

		lifecycle.attach({
			id: 'ws::term',
			handle,
			container,
			active: true,
		});

		expect(container.firstChild).toBe(handle.container);
		expect(handle.terminal.open).toHaveBeenCalledTimes(1);
		expect(ensureOverlay).toHaveBeenCalledWith(handle, 'ws::term');
		expect(loadRendererAddon).toHaveBeenCalledWith('ws::term', handle);
		expect(scheduleFitStabilization).toHaveBeenCalledWith('ws::term', 'open');
		expect(fitWithPreservedViewport).toHaveBeenCalledTimes(1);
		expect(resizeToFit).toHaveBeenCalledTimes(1);
		expect(handle.terminal.focus).toHaveBeenCalledTimes(1);
		expect(flushOutput).toHaveBeenCalledWith('ws::term', false);
		expect(markAttached).toHaveBeenCalledWith('ws::term');

		resolveFonts?.();
		await flushPromises();

		expect(fitWithPreservedViewport).toHaveBeenCalledTimes(2);
		expect(resizeToFit).toHaveBeenCalledTimes(2);
	});

	it('skips open path when terminal is already opened in host container', () => {
		const container = document.createElement('div') as HTMLDivElement;
		const handle = createHandle();
		handle.terminal.element = { parentElement: handle.container };
		container.replaceChildren(handle.container);
		const ensureOverlay = vi.fn();
		const loadRendererAddon = vi.fn();
		const fitWithPreservedViewport = vi.fn();
		const resizeToFit = vi.fn();
		const scheduleFitStabilization = vi.fn();
		const flushOutput = vi.fn();
		const markAttached = vi.fn();
		const lifecycle = createTerminalAttachOpenLifecycle({
			getHandle: () => handle,
			ensureOverlay,
			loadRendererAddon,
			fitWithPreservedViewport,
			resizeToFit,
			scheduleFitStabilization,
			flushOutput,
			markAttached,
			resolveFontsReady: () => Promise.resolve(),
		});

		lifecycle.attach({
			id: 'ws::term',
			handle,
			container,
			active: false,
		});

		expect(handle.terminal.open).not.toHaveBeenCalled();
		expect(ensureOverlay).not.toHaveBeenCalled();
		expect(loadRendererAddon).not.toHaveBeenCalled();
		expect(scheduleFitStabilization).not.toHaveBeenCalled();
		expect(handle.terminal.focus).not.toHaveBeenCalled();
		expect(fitWithPreservedViewport).toHaveBeenCalledTimes(1);
		expect(resizeToFit).toHaveBeenCalledTimes(1);
		expect(flushOutput).toHaveBeenCalledWith('ws::term', false);
		expect(markAttached).toHaveBeenCalledWith('ws::term');
	});

	it('does not run delayed font fit when terminal handle is gone', async () => {
		const container = document.createElement('div') as HTMLDivElement;
		const handle = createHandle();
		let resolveFonts: (() => void) | undefined;
		const fontsReady = new Promise<void>((resolve) => {
			resolveFonts = resolve;
		});
		const fitWithPreservedViewport = vi.fn();
		const resizeToFit = vi.fn();
		const getHandle = vi.fn(() => undefined);
		const lifecycle = createTerminalAttachOpenLifecycle({
			getHandle,
			ensureOverlay: vi.fn(),
			loadRendererAddon: vi.fn(),
			fitWithPreservedViewport,
			resizeToFit,
			scheduleFitStabilization: vi.fn(),
			flushOutput: vi.fn(),
			markAttached: vi.fn(),
			resolveFontsReady: () => fontsReady,
		});

		lifecycle.attach({
			id: 'ws::term',
			handle,
			container,
			active: false,
		});

		resolveFonts?.();
		await flushPromises();

		expect(getHandle).toHaveBeenCalledWith('ws::term');
		expect(fitWithPreservedViewport).toHaveBeenCalledTimes(1);
		expect(resizeToFit).toHaveBeenCalledTimes(1);
	});
});
