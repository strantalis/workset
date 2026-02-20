import { describe, expect, it, vi } from 'vitest';
import { createTerminalSyncController } from './terminalSyncController';

describe('terminalSyncController', () => {
	it('attaches, observes, and syncs stream for mounted terminals', () => {
		const ensureGlobals = vi.fn();
		const ensureContext = vi.fn((input) => input);
		const attachTerminal = vi.fn();
		const attachResizeObserver = vi.fn();
		const syncTerminalStream = vi.fn();
		const focusTerminal = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals,
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
			focusTerminal,
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

		expect(ensureGlobals).toHaveBeenCalledTimes(1);
		expect(ensureContext).toHaveBeenCalledWith({
			terminalKey: 'ws::term',
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});
		expect(attachTerminal).toHaveBeenCalledWith('ws::term', container, true);
		expect(attachResizeObserver).toHaveBeenCalledWith('ws::term', container);
		expect(focusTerminal).toHaveBeenCalledWith('ws::term');
		expect(syncTerminalStream).toHaveBeenCalledWith('ws::term');
	});

	it('skips attach and stream when container is missing', () => {
		const attachTerminal = vi.fn();
		const syncTerminalStream = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			deleteContext: vi.fn(),
			attachTerminal,
			attachResizeObserver: vi.fn(),
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

		expect(attachTerminal).not.toHaveBeenCalled();
		expect(syncTerminalStream).not.toHaveBeenCalled();
	});

	it('skips unchanged sync payloads', () => {
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

		expect(attachTerminal).toHaveBeenCalledTimes(1);
		expect(attachResizeObserver).toHaveBeenCalledTimes(1);
		expect(syncTerminalStream).toHaveBeenCalledTimes(1);
	});

	it('re-attaches and re-syncs stream on container churn', () => {
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
		const first = document.createElement('div') as HTMLDivElement;
		const second = document.createElement('div') as HTMLDivElement;

		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container: first,
			active: true,
		});
		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container: second,
			active: true,
		});

		expect(attachTerminal).toHaveBeenCalledTimes(2);
		expect(attachResizeObserver).toHaveBeenCalledTimes(2);
		expect(syncTerminalStream).toHaveBeenCalledTimes(2);
	});

	it('treats active-only flips as attach+focus updates with stream reassert on activation', () => {
		const attachTerminal = vi.fn();
		const attachResizeObserver = vi.fn();
		const syncTerminalStream = vi.fn();
		const focusTerminal = vi.fn();
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
			focusTerminal,
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
			trace,
		});
		const container = document.createElement('div') as HTMLDivElement;

		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: false,
			source: 'controller.initial',
		});
		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
			source: 'controller.active_change',
		});

		expect(attachTerminal).toHaveBeenCalledTimes(2);
		expect(attachResizeObserver).toHaveBeenCalledTimes(2);
		expect(syncTerminalStream).toHaveBeenCalledTimes(2);
		expect(focusTerminal).toHaveBeenCalledWith('ws::term');
		expect(trace).toHaveBeenCalledWith(
			'ws::term',
			'sync_terminal_active_flip_attach',
			expect.objectContaining({
				previousActive: false,
				active: true,
			}),
		);
	});

	it('keeps attach updates on active -> inactive flips for the same container', () => {
		const attachTerminal = vi.fn();
		const attachResizeObserver = vi.fn();
		const syncTerminalStream = vi.fn();
		const focusTerminal = vi.fn();
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
			focusTerminal,
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
			source: 'controller.initial',
		});
		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: false,
			source: 'controller.active_change',
		});

		expect(attachTerminal).toHaveBeenCalledTimes(2);
		expect(attachResizeObserver).toHaveBeenCalledTimes(2);
		expect(syncTerminalStream).toHaveBeenCalledTimes(1);
		expect(focusTerminal).toHaveBeenCalledTimes(1);
		expect(trace).toHaveBeenCalledWith(
			'ws::term',
			'sync_terminal_active_flip_attach',
			expect.objectContaining({
				previousActive: true,
				active: false,
			}),
		);
	});

	it('detaches displaced terminal when another terminal reuses the same container', () => {
		const attachTerminal = vi.fn();
		const detachResizeObserver = vi.fn();
		const markDetached = vi.fn();
		const ensureContext = vi.fn((input) => input);
		const trace = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext,
			deleteContext: vi.fn(),
			attachTerminal,
			attachResizeObserver: vi.fn(),
			detachResizeObserver,
			syncTerminalStream: vi.fn(),
			markDetached,
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
			terminalId: 'term-a',
			container,
			active: true,
		});
		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term-b',
			container,
			active: true,
		});

		expect(attachTerminal).toHaveBeenCalledWith('ws::term-a', container, true);
		expect(attachTerminal).toHaveBeenCalledWith('ws::term-b', container, true);
		expect(markDetached).toHaveBeenCalledWith('ws::term-a');
		expect(detachResizeObserver).toHaveBeenCalledWith('ws::term-a');
		expect(ensureContext).toHaveBeenCalledWith({
			terminalKey: 'ws::term-a',
			workspaceId: 'ws',
			terminalId: 'term-a',
			container: null,
			active: false,
		});
		expect(trace).toHaveBeenCalledWith(
			'ws::term-a',
			'sync_terminal_displaced_by_container_reuse',
			expect.objectContaining({
				by: 'ws::term-b',
			}),
		);
	});

	it('skips stale detach while the connected container is still live', () => {
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
			controller.detachTerminal('ws', 'term');
			expect(markDetached).not.toHaveBeenCalled();
		} finally {
			container.remove();
		}
	});

	it('forces detach even when container remains connected', () => {
		const markDetached = vi.fn();
		const detachResizeObserver = vi.fn();
		const ensureContext = vi.fn((input) => input);
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext,
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
			controller.detachTerminal('ws', 'term', { force: true });
			expect(markDetached).toHaveBeenCalledWith('ws::term');
			expect(detachResizeObserver).toHaveBeenCalledWith('ws::term');
			expect(ensureContext).toHaveBeenCalledWith({
				terminalKey: 'ws::term',
				workspaceId: 'ws',
				terminalId: 'term',
				container: null,
				active: false,
			});
		} finally {
			container.remove();
		}
	});

	it('closes terminal by detaching, stopping, disposing, and deleting context', async () => {
		const markDetached = vi.fn();
		const detachResizeObserver = vi.fn();
		const stopTerminal = vi.fn(async () => undefined);
		const disposeTerminalResources = vi.fn();
		const deleteContext = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
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
		await controller.closeTerminal('ws', 'term');

		expect(markDetached).toHaveBeenCalledWith('ws::term');
		expect(detachResizeObserver).toHaveBeenCalledWith('ws::term');
		expect(stopTerminal).toHaveBeenCalledWith('ws', 'term');
		expect(disposeTerminalResources).toHaveBeenCalledWith('ws::term');
		expect(deleteContext).toHaveBeenCalledWith('ws::term');
	});

	it('still disposes and deletes context when stop fails', async () => {
		const disposeTerminalResources = vi.fn();
		const deleteContext = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			deleteContext,
			attachTerminal: vi.fn(),
			attachResizeObserver: vi.fn(),
			detachResizeObserver: vi.fn(),
			syncTerminalStream: vi.fn(),
			markDetached: vi.fn(),
			stopTerminal: vi.fn(async () => {
				throw new Error('terminal not found');
			}),
			disposeTerminalResources,
			focusTerminal: vi.fn(),
			scrollToBottom: vi.fn(),
			isAtBottom: vi.fn(() => true),
		});

		await expect(controller.closeTerminal('ws', 'term')).resolves.toBeUndefined();
		expect(disposeTerminalResources).toHaveBeenCalledWith('ws::term');
		expect(deleteContext).toHaveBeenCalledWith('ws::term');
	});

	it('clears stale local state when context registry no longer has the terminal key', () => {
		let hasContext = true;
		const attachTerminal = vi.fn();
		const syncTerminalStream = vi.fn();
		const trace = vi.fn();
		const controller = createTerminalSyncController({
			ensureGlobals: vi.fn(),
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			ensureContext: (input) => input,
			hasContext: () => hasContext,
			deleteContext: vi.fn(),
			attachTerminal,
			attachResizeObserver: vi.fn(),
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
		const first = document.createElement('div') as HTMLDivElement;
		const second = document.createElement('div') as HTMLDivElement;

		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container: first,
			active: true,
		});

		hasContext = false;
		controller.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container: second,
			active: true,
		});

		expect(attachTerminal).toHaveBeenCalledTimes(2);
		expect(syncTerminalStream).toHaveBeenCalledTimes(2);
		expect(trace).toHaveBeenCalledWith(
			'ws::term',
			'sync_terminal_clear_stale_state',
			expect.objectContaining({ source: 'unspecified' }),
		);
	});
});
