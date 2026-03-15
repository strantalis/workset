import { describe, expect, it, vi } from 'vitest';
import { createTerminalInputOrchestrator } from './terminalInputOrchestrator';

const buildDeps = () => ({
	ensureSessionActive: vi.fn(async () => undefined),
	hasStarted: vi.fn(() => true),
	appendPendingInput: vi.fn(),
	recordOutputBytes: vi.fn(),
	getWorkspaceId: vi.fn(() => 'ws'),
	getTerminalId: vi.fn(() => 'term'),
	write: vi.fn(async () => undefined),
	markStopped: vi.fn(),
});

describe('terminalInputOrchestrator', () => {
	it('returns immediately for empty input', () => {
		const deps = buildDeps();
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', '');

		expect(deps.ensureSessionActive).not.toHaveBeenCalled();
		expect(deps.appendPendingInput).not.toHaveBeenCalled();
		expect(deps.write).not.toHaveBeenCalled();
	});

	it('queues input when terminal has not started', () => {
		const deps = buildDeps();
		deps.hasStarted.mockReturnValue(false);
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', 'input');

		expect(deps.ensureSessionActive).toHaveBeenCalledWith('ws::term');
		expect(deps.appendPendingInput).toHaveBeenCalledWith('ws::term', 'input');
		expect(deps.write).not.toHaveBeenCalled();
		expect(deps.recordOutputBytes).not.toHaveBeenCalled();
	});

	it('writes input when terminal is started', () => {
		const deps = buildDeps();
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', 'input');

		expect(deps.ensureSessionActive).not.toHaveBeenCalled();
		expect(deps.recordOutputBytes).toHaveBeenCalledWith('ws::term', 5);
		expect(deps.write).toHaveBeenCalledWith('ws', 'term', 'input');
	});

	it('queues input and marks terminal stopped when write fails', async () => {
		const deps = buildDeps();
		deps.write.mockRejectedValue('session not found');
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', 'input');
		await Promise.resolve();
		await Promise.resolve();

		expect(deps.appendPendingInput).toHaveBeenCalledWith('ws::term', 'input');
		expect(deps.markStopped).toHaveBeenCalledWith('ws::term');
		expect(deps.ensureSessionActive).toHaveBeenCalledWith('ws::term');
	});

	it('drops terminal focus reports so they do not leak into the shell', () => {
		const deps = buildDeps();
		const trace = vi.fn();
		const orchestrator = createTerminalInputOrchestrator({
			...deps,
			trace,
		});

		orchestrator.sendInput('ws::term', '\x1b[I');
		orchestrator.sendInput('ws::term', '\x1b[O');

		expect(deps.ensureSessionActive).not.toHaveBeenCalled();
		expect(deps.appendPendingInput).not.toHaveBeenCalled();
		expect(deps.recordOutputBytes).not.toHaveBeenCalled();
		expect(deps.write).not.toHaveBeenCalled();
		expect(trace).toHaveBeenCalledTimes(2);
	});

	it('strips focus reports embedded inside normal input chunks', () => {
		const deps = buildDeps();
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', '\x1b[I\x1b[O\x03');

		expect(deps.recordOutputBytes).toHaveBeenCalledWith('ws::term', 1);
		expect(deps.write).toHaveBeenCalledWith('ws', 'term', '\x03');
	});

	it('sends protocol responses over the live path', () => {
		const deps = buildDeps();
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendProtocolResponse('ws::term', '\x1b]10;rgb:1a1a/2b2b/3c3c\x1b\\');

		expect(deps.recordOutputBytes).toHaveBeenCalledWith('ws::term', 25);
		expect(deps.write).toHaveBeenCalledWith('ws', 'term', '\x1b]10;rgb:1a1a/2b2b/3c3c\x1b\\');
	});
});
