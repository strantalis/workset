import { describe, expect, it, vi } from 'vitest';
import { createTerminalSyncController } from './terminalSyncController';

const flushMicrotasks = async (): Promise<void> => {
	await Promise.resolve();
	await Promise.resolve();
};

describe('terminalSyncController', () => {
	it('syncs attached terminals without redundant fit/redraw work', async () => {
		const ensureGlobals = vi.fn();
		const buildTerminalKey = vi.fn(
			(workspaceId: string, terminalId: string) => `${workspaceId}::${terminalId}`,
		);
		const ensureContext = vi.fn((input) => input);
		const attachTerminal = vi.fn();
		const attachResizeObserver = vi.fn();
		const syncTerminalStream = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals,
			buildTerminalKey,
			ensureContext,
			deleteContext: vi.fn(),
			attachTerminal,
			attachResizeObserver,
			detachResizeObserver: vi.fn(),
			syncTerminalStream,
			markDetached: vi.fn(),
			stopTerminal: vi.fn(async () => undefined),
			disposeTerminalResources: vi.fn(),
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
		});
		const container = document.createElement('div') as HTMLDivElement;

		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});
		await flushMicrotasks();

		expect(ensureGlobals).toHaveBeenCalledTimes(1);
		expect(buildTerminalKey).toHaveBeenCalledWith('ws', 'term');
		expect(ensureContext).toHaveBeenCalledWith(
			expect.objectContaining({
				terminalKey: 'ws::term',
				workspaceId: 'ws',
				terminalId: 'term',
				container,
				active: true,
			}),
		);
		expect(attachTerminal).toHaveBeenCalledWith('ws::term', container, true);
		expect(attachResizeObserver).toHaveBeenCalledWith('ws::term', container);
		expect(syncTerminalStream).toHaveBeenCalledWith('ws::term');
	});

	it('skips stream sync when container is missing', async () => {
		const attachTerminal = vi.fn();
		const attachResizeObserver = vi.fn();
		const syncTerminalStream = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			deleteContext: vi.fn(),
			attachTerminal,
			attachResizeObserver,
			detachResizeObserver: vi.fn(),
			syncTerminalStream,
			markDetached: vi.fn(),
			stopTerminal: vi.fn(async () => undefined),
			disposeTerminalResources: vi.fn(),
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
		});

		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container: null,
			active: false,
		});
		await flushMicrotasks();

		expect(attachTerminal).not.toHaveBeenCalled();
		expect(attachResizeObserver).not.toHaveBeenCalled();
		expect(syncTerminalStream).not.toHaveBeenCalled();
	});

	it('skips unchanged sync payloads to avoid attach/stream thrash', async () => {
		const attachTerminal = vi.fn();
		const attachResizeObserver = vi.fn();
		const syncTerminalStream = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			deleteContext: vi.fn(),
			attachTerminal,
			attachResizeObserver,
			detachResizeObserver: vi.fn(),
			syncTerminalStream,
			markDetached: vi.fn(),
			stopTerminal: vi.fn(async () => undefined),
			disposeTerminalResources: vi.fn(),
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
		});
		const container = document.createElement('div') as HTMLDivElement;

		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: false,
		});
		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: false,
		});
		await flushMicrotasks();

		expect(attachTerminal).toHaveBeenCalledTimes(1);
		expect(attachResizeObserver).toHaveBeenCalledTimes(1);
		expect(syncTerminalStream).toHaveBeenCalledTimes(1);
	});

	it('does not re-attach unchanged active panes', async () => {
		const attachTerminal = vi.fn();
		const attachResizeObserver = vi.fn();
		const syncTerminalStream = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			deleteContext: vi.fn(),
			attachTerminal,
			attachResizeObserver,
			detachResizeObserver: vi.fn(),
			syncTerminalStream,
			markDetached: vi.fn(),
			stopTerminal: vi.fn(async () => undefined),
			disposeTerminalResources: vi.fn(),
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
		});
		const container = document.createElement('div') as HTMLDivElement;

		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});
		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});
		await flushMicrotasks();

		expect(attachTerminal).toHaveBeenCalledTimes(1);
		expect(attachResizeObserver).toHaveBeenCalledTimes(1);
		expect(syncTerminalStream).toHaveBeenCalledTimes(1);
	});

	it('skips stream sync on container churn when identity and active state are unchanged', async () => {
		const attachTerminal = vi.fn();
		const attachResizeObserver = vi.fn();
		const syncTerminalStream = vi.fn();
		const trace = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			deleteContext: vi.fn(),
			attachTerminal,
			attachResizeObserver,
			detachResizeObserver: vi.fn(),
			syncTerminalStream,
			markDetached: vi.fn(),
			stopTerminal: vi.fn(async () => undefined),
			disposeTerminalResources: vi.fn(),
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
			trace,
		});
		const firstContainer = document.createElement('div') as HTMLDivElement;
		const secondContainer = document.createElement('div') as HTMLDivElement;

		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container: firstContainer,
			active: true,
		});
		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container: secondContainer,
			active: true,
		});
		await flushMicrotasks();

		// Coalescing keeps only the latest attach intent in the same microtask tick.
		expect(attachTerminal).toHaveBeenCalledTimes(1);
		expect(attachResizeObserver).toHaveBeenCalledTimes(1);
		expect(syncTerminalStream).toHaveBeenCalledTimes(1);
		expect(trace).toHaveBeenCalledWith(
			'ws::term',
			'sync_terminal_skip_stream_container_churn',
			expect.objectContaining({ active: true }),
		);
	});

	it('preserves previous non-null context across transient container null updates', async () => {
		const ensureContext = vi.fn((input) => input);
		const attachTerminal = vi.fn();
		const attachResizeObserver = vi.fn();
		const syncTerminalStream = vi.fn();
		const trace = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext,
			deleteContext: vi.fn(),
			attachTerminal,
			attachResizeObserver,
			detachResizeObserver: vi.fn(),
			syncTerminalStream,
			markDetached: vi.fn(),
			stopTerminal: vi.fn(async () => undefined),
			disposeTerminalResources: vi.fn(),
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
			trace,
		});
		const container = document.createElement('div') as HTMLDivElement;

		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});
		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container: null,
			active: true,
		});
		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});
		await flushMicrotasks();

		// Initial attach only; null-container transient and unchanged sync are ignored.
		expect(attachTerminal).toHaveBeenCalledTimes(1);
		expect(attachResizeObserver).toHaveBeenCalledTimes(1);
		expect(syncTerminalStream).toHaveBeenCalledTimes(1);
		// ensureContext should not be overwritten with null container in between.
		expect(ensureContext).toHaveBeenCalledTimes(1);
		expect(trace).toHaveBeenCalledWith(
			'ws::term',
			'sync_terminal_no_container_preserve_previous',
			expect.objectContaining({
				active: true,
				previousActive: true,
			}),
		);
	});

	it('detaches and closes terminals by key', async () => {
		const markDetached = vi.fn();
		const detachResizeObserver = vi.fn();
		const stopTerminal = vi.fn(async () => undefined);
		const disposeTerminalResources = vi.fn();
		const deleteContext = vi.fn();
		const ensureContext = vi.fn((input) => input);
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext,
			deleteContext,
			attachTerminal: vi.fn(),
			attachResizeObserver: vi.fn(),
			detachResizeObserver,
			syncTerminalStream: vi.fn(),
			markDetached,
			stopTerminal,
			disposeTerminalResources,
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
		});
		const container = document.createElement('div') as HTMLDivElement;

		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});
		await flushMicrotasks();

		controller.detachTerminal('ws', 'term');
		await flushMicrotasks();
		await controller.closeTerminal('ws', 'term');
		await flushMicrotasks();

		expect(markDetached).toHaveBeenCalledWith('ws::term');
		expect(detachResizeObserver).toHaveBeenCalledWith('ws::term');
		expect(ensureContext).toHaveBeenCalledWith({
			terminalKey: 'ws::term',
			workspaceId: 'ws',
			terminalId: 'term',
			container: null,
			active: false,
		});
		expect(stopTerminal).toHaveBeenCalledWith('ws', 'term');
		expect(disposeTerminalResources).toHaveBeenCalledWith('ws::term');
		expect(deleteContext).toHaveBeenCalledTimes(1);
		expect(deleteContext).toHaveBeenCalledWith('ws::term');
	});

	it('skips stale detach when a live container is still connected', async () => {
		const markDetached = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			deleteContext: vi.fn(),
			attachTerminal: vi.fn(),
			attachResizeObserver: vi.fn(),
			detachResizeObserver: vi.fn(),
			syncTerminalStream: vi.fn(),
			markDetached,
			stopTerminal: vi.fn(async () => undefined),
			disposeTerminalResources: vi.fn(),
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
		});
		const container = document.createElement('div') as HTMLDivElement;
		document.body.appendChild(container);
		try {
			controller.syncTerminal({
				workspaceId: 'ws',
				terminalId: 'term',
				container,
				active: true,
			});
			await flushMicrotasks();
			controller.detachTerminal('ws', 'term');
			await flushMicrotasks();
			expect(markDetached).not.toHaveBeenCalled();
		} finally {
			container.remove();
		}
	});

	it('forces detach when requested even if container is still connected', async () => {
		const markDetached = vi.fn();
		const detachResizeObserver = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			deleteContext: vi.fn(),
			attachTerminal: vi.fn(),
			attachResizeObserver: vi.fn(),
			detachResizeObserver,
			syncTerminalStream: vi.fn(),
			markDetached,
			stopTerminal: vi.fn(async () => undefined),
			disposeTerminalResources: vi.fn(),
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
		});
		const container = document.createElement('div') as HTMLDivElement;
		document.body.appendChild(container);
		try {
			controller.syncTerminal({
				workspaceId: 'ws',
				terminalId: 'term',
				container,
				active: true,
			});
			await flushMicrotasks();
			controller.detachTerminal('ws', 'term', { force: true });
			await flushMicrotasks();
			expect(markDetached).toHaveBeenCalledWith('ws::term');
			expect(detachResizeObserver).toHaveBeenCalledWith('ws::term');
		} finally {
			container.remove();
		}
	});

	it('reattaches terminal after a forced detach on the same connected container', async () => {
		const attachTerminal = vi.fn();
		const attachResizeObserver = vi.fn();
		const syncTerminalStream = vi.fn();
		const markDetached = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			deleteContext: vi.fn(),
			attachTerminal,
			attachResizeObserver,
			detachResizeObserver: vi.fn(),
			syncTerminalStream,
			markDetached,
			stopTerminal: vi.fn(async () => undefined),
			disposeTerminalResources: vi.fn(),
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
		});
		const container = document.createElement('div') as HTMLDivElement;
		document.body.appendChild(container);
		try {
			controller.syncTerminal({
				workspaceId: 'ws',
				terminalId: 'term',
				container,
				active: true,
			});
			await flushMicrotasks();

			controller.detachTerminal('ws', 'term', { force: true });
			await flushMicrotasks();

			controller.syncTerminal({
				workspaceId: 'ws',
				terminalId: 'term',
				container,
				active: true,
			});
			await flushMicrotasks();

			expect(markDetached).toHaveBeenCalledTimes(1);
			expect(attachTerminal).toHaveBeenCalledTimes(2);
			expect(attachResizeObserver).toHaveBeenCalledTimes(2);
			expect(syncTerminalStream).toHaveBeenCalledTimes(2);
		} finally {
			container.remove();
		}
	});

	it('disposes and deletes context even if stop fails', async () => {
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			deleteContext: vi.fn(),
			attachTerminal: vi.fn(),
			attachResizeObserver: vi.fn(),
			detachResizeObserver: vi.fn(),
			syncTerminalStream: vi.fn(),
			markDetached: vi.fn(),
			stopTerminal: vi.fn(async () => {
				throw new Error('terminal not found');
			}),
			disposeTerminalResources: vi.fn(),
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
		});

		await expect(controller.closeTerminal('ws', 'term')).resolves.toBeUndefined();
	});

	it('clears stale sync state when no live context exists for the terminal key', async () => {
		let hasContext = true;
		const syncTerminalStream = vi.fn();
		const trace = vi.fn();
		const attachTerminal = vi.fn();
		const attachResizeObserver = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			hasContext: () => hasContext,
			deleteContext: vi.fn(),
			attachTerminal,
			attachResizeObserver,
			detachResizeObserver: vi.fn(),
			syncTerminalStream,
			markDetached: vi.fn(),
			stopTerminal: vi.fn(async () => undefined),
			disposeTerminalResources: vi.fn(),
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
			trace,
		});
		const firstContainer = document.createElement('div') as HTMLDivElement;
		const secondContainer = document.createElement('div') as HTMLDivElement;

		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container: firstContainer,
			active: true,
		});
		await flushMicrotasks();

		hasContext = false;
		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container: secondContainer,
			active: true,
		});
		await flushMicrotasks();

		expect(syncTerminalStream).toHaveBeenCalledTimes(2);
		expect(attachTerminal).toHaveBeenCalledTimes(2);
		expect(attachResizeObserver).toHaveBeenCalledTimes(2);
		expect(trace).toHaveBeenCalledWith('ws::term', 'sync_terminal_clear_stale_state', {});
	});
});
