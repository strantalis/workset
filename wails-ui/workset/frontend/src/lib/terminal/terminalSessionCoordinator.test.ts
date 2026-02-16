import { describe, expect, it, vi } from 'vitest';
import { createTerminalSessionCoordinator } from './terminalSessionCoordinator';

const createCoordinator = (input?: {
	start?: () => Promise<void>;
	write?: () => Promise<void>;
	pendingInput?: Map<string, string>;
	onSessionReady?: (id: string) => void;
}) => {
	const started = new Set<string>();
	const startInFlight = new Set<string>();
	const setStatusAndMessage = vi.fn();
	const setInput = vi.fn();
	const ensureRendererDefaults = vi.fn();
	const emitState = vi.fn();
	const setHealth = vi.fn();
	const clearStartupTimeout = vi.fn();
	const clearStartInFlight = vi.fn((id: string) => startInFlight.delete(id));
	const pendingInput = input?.pendingInput ?? new Map<string, string>();
	const logDebug = vi.fn();
	const transport = {
		start: vi.fn(input?.start ?? (async () => undefined)),
		write: vi.fn(input?.write ?? (async () => undefined)),
		fetchSettings: vi.fn(async () => null),
		fetchSessiondStatus: vi.fn(async () => null),
	};

	const coordinator = createTerminalSessionCoordinator({
		lifecycle: {
			hasStarted: (id) => started.has(id),
			hasStartInFlight: (id) => startInFlight.has(id),
			isSessiondAvailable: () => true,
			markStarted: (id) => started.add(id),
			markStopped: (id) => started.delete(id),
			setStatusAndMessage,
			setInput,
			ensureRendererDefaults,
			markStartInFlight: (id) => startInFlight.add(id),
			clearStartInFlight,
			clearStartupTimeout,
			dropHealthCheck: vi.fn(),
			setSessiondStatus: vi.fn(),
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
		onSessionReady: input?.onSessionReady,
	});

	return {
		coordinator,
		transport,
		pendingInput,
		emitState,
		setStatusAndMessage,
		setInput,
		setHealth,
		ensureRendererDefaults,
		clearStartupTimeout,
		clearStartInFlight,
		logDebug,
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

	it('reasserts start when begin is called repeatedly for a started terminal', async () => {
		const { coordinator, transport, logDebug } = createCoordinator();
		await coordinator.beginTerminal('ws::term');
		await coordinator.beginTerminal('ws::term');

		expect(transport.start).toHaveBeenCalledTimes(2);
		expect(logDebug).toHaveBeenCalledWith(
			'ws::term',
			'session_begin_reassert_ok',
			expect.objectContaining({
				quiet: false,
			}),
		);
	});

	it('notifies when session becomes ready', async () => {
		const onSessionReady = vi.fn();
		const { coordinator } = createCoordinator({ onSessionReady });

		await coordinator.beginTerminal('ws::term');

		expect(onSessionReady).toHaveBeenCalledWith('ws::term');
	});
});
