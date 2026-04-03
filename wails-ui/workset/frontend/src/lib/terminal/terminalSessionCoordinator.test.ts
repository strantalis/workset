import { describe, expect, it, vi } from 'vitest';
import { createTerminalSessionCoordinator } from './terminalSessionCoordinator';
import type { TerminalSessionStartResult } from './terminalTransport';

const defaultDescriptor: TerminalSessionStartResult = {
	workspaceId: 'ws',
	terminalId: 'term',
	sessionId: 'ws::term',
	socketUrl: 'ws://127.0.0.1:9001/stream',
	socketToken: 'token',
};

const createCoordinator = (input?: {
	start?: () => Promise<TerminalSessionStartResult>;
	write?: () => Promise<void>;
	pendingInput?: Map<string, string>;
	onSessionReady?: (id: string, descriptor: TerminalSessionStartResult) => void;
	startupTimeoutMs?: number;
}) => {
	const started = new Set<string>();
	const startInFlight = new Set<string>();
	const setStatusAndMessage = vi.fn();
	const setInput = vi.fn();
	const emitState = vi.fn();
	const setHealth = vi.fn();
	const clearStartupTimeout = vi.fn();
	const clearStartInFlight = vi.fn((id: string) => startInFlight.delete(id));
	const pendingInput = input?.pendingInput ?? new Map<string, string>();
	const logDebug = vi.fn();
	const setCurrentTerminalFontSize = vi.fn();
	const setCurrentCursorBlink = vi.fn();
	const transport = {
		start: vi.fn(input?.start ?? (async () => defaultDescriptor)),
		write: vi.fn(input?.write ?? (async () => undefined)),
		fetchSettings: vi.fn(async () => null),
		fetchTerminalServiceStatus: vi.fn(async () => null),
	};

	const coordinator = createTerminalSessionCoordinator({
		lifecycle: {
			hasStarted: (id) => started.has(id),
			hasStartInFlight: (id) => startInFlight.has(id),
			isTerminalServiceAvailable: () => true,
			markStarted: (id) => started.add(id),
			markStopped: (id) => started.delete(id),
			setStatusAndMessage,
			setInput,
			markStartInFlight: (id) => startInFlight.add(id),
			clearStartInFlight,
			clearStartupTimeout,
			dropHealthCheck: vi.fn(),
			setTerminalServiceStatus: vi.fn(),
		},
		getWorkspaceId: () => 'ws',
		getTerminalId: () => 'term',
		transport,
		setHealth,
		emitState,
		pendingInput,
		logDebug,
		resetSessionState: vi.fn(),
		writeStartFailureMessage: vi.fn(),
		getDebugOverlayPreference: () => '',
		setDebugOverlayPreference: vi.fn(),
		clearLocalDebugPreference: vi.fn(),
		syncDebugEnabled: vi.fn(),
		getCurrentTerminalFontSize: () => 13,
		setCurrentTerminalFontSize,
		getCurrentCursorBlink: () => true,
		setCurrentCursorBlink,
		onSessionReady: input?.onSessionReady,
		startupTimeoutMs: input?.startupTimeoutMs,
	});

	return {
		coordinator,
		transport,
		pendingInput,
		emitState,
		setStatusAndMessage,
		setInput,
		setHealth,
		clearStartupTimeout,
		clearStartInFlight,
		logDebug,
		setCurrentTerminalFontSize,
		setCurrentCursorBlink,
	};
};

describe('terminalSessionCoordinator', () => {
	it('flushes queued input after start succeeds', async () => {
		const pendingInput = new Map<string, string>([['ws::term', 'opencode\n']]);
		const { coordinator, transport } = createCoordinator({ pendingInput });

		await coordinator.beginTerminal('ws::term');

		expect(transport.start).toHaveBeenCalledWith('ws', 'term');
		expect(transport.write).toHaveBeenCalledWith('ws', 'term', 'opencode\n');
		expect(pendingInput.has('ws::term')).toBe(false);
	});

	it('re-queues pending input if flush write fails', async () => {
		const pendingInput = new Map<string, string>([['ws::term', 'opencode\n']]);
		const { coordinator, transport } = createCoordinator({
			pendingInput,
			write: async () => {
				throw new Error('write failed');
			},
		});

		await coordinator.beginTerminal('ws::term');

		expect(transport.start).toHaveBeenCalledWith('ws', 'term');
		expect(transport.write).toHaveBeenCalledWith('ws', 'term', 'opencode\n');
		expect(pendingInput.get('ws::term')).toBe('opencode\n');
	});

	it('logs skip reason for quiet ensure when terminal is already started', async () => {
		const { coordinator, logDebug } = createCoordinator();

		await coordinator.beginTerminal('ws::term');
		await coordinator.ensureSessionActive('ws::term');

		expect(logDebug).toHaveBeenCalledWith(
			'ws::term',
			'session_begin_skip_started',
			expect.objectContaining({ quiet: true }),
		);
	});

	it('skips restart when begin is called repeatedly for a started terminal', async () => {
		const { coordinator, transport, logDebug } = createCoordinator();
		await coordinator.beginTerminal('ws::term');
		await coordinator.beginTerminal('ws::term');

		expect(transport.start).toHaveBeenCalledTimes(1);
		expect(logDebug).toHaveBeenCalledWith(
			'ws::term',
			'session_begin_skip_started',
			expect.objectContaining({
				quiet: false,
			}),
		);
	});

	it('notifies when session becomes ready', async () => {
		const onSessionReady = vi.fn();
		const { coordinator } = createCoordinator({ onSessionReady });

		await coordinator.beginTerminal('ws::term');

		expect(onSessionReady).toHaveBeenCalledWith('ws::term', defaultDescriptor);
	});

	it('loads terminal appearance defaults from settings', async () => {
		const { coordinator, transport, setCurrentTerminalFontSize, setCurrentCursorBlink } =
			createCoordinator();
		transport.fetchSettings.mockResolvedValue({
			defaults: {
				terminalDebugOverlay: 'off',
				terminalFontSize: '16',
				terminalCursorBlink: 'off',
			},
		} as never);

		await coordinator.loadTerminalDefaults();

		expect(setCurrentTerminalFontSize).toHaveBeenCalledWith(16);
		expect(setCurrentCursorBlink).toHaveBeenCalledWith(false);
	});

	it('fails startup when bootstrap hangs past the timeout', async () => {
		vi.useFakeTimers();
		const { coordinator, setStatusAndMessage, setHealth, clearStartInFlight } = createCoordinator({
			start: () => new Promise<TerminalSessionStartResult>(() => undefined),
			startupTimeoutMs: 25,
		});

		const startPromise = coordinator.beginTerminal('ws::term');
		await vi.advanceTimersByTimeAsync(30);
		await startPromise;

		expect(setStatusAndMessage).toHaveBeenCalledWith(
			'ws::term',
			'error',
			'Error: Terminal startup timed out.',
		);
		expect(setHealth).toHaveBeenCalledWith('ws::term', 'stale', 'Failed to start terminal.');
		expect(clearStartInFlight).toHaveBeenCalledWith('ws::term');
	});
});
