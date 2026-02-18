import { describe, expect, it, vi } from 'vitest';
import { createTerminalInputOrchestrator } from './terminalInputOrchestrator';

const buildDeps = () => ({
	ensureSessionActive: vi.fn(async () => undefined),
	hasStarted: vi.fn(() => true),
	appendPendingInput: vi.fn(),
	recordOutputBytes: vi.fn(),
	getWorkspaceId: vi.fn(() => 'ws'),
	getTerminalId: vi.fn(() => 'term'),
	isContextActive: vi.fn(() => true),
	isTerminalFocused: vi.fn(() => true),
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

	it('drops printable input from inactive terminal contexts', () => {
		const deps = buildDeps();
		deps.isContextActive.mockReturnValue(false);
		const trace = vi.fn();
		const orchestrator = createTerminalInputOrchestrator({
			...deps,
			trace,
		});

		orchestrator.sendInput('ws::term', 'opencode');

		expect(deps.write).not.toHaveBeenCalled();
		expect(deps.appendPendingInput).not.toHaveBeenCalled();
		expect(trace).toHaveBeenCalledWith(
			'ws::term',
			'frontend_input_drop_inactive_context',
			expect.objectContaining({ bytes: 8 }),
		);
	});

	it('drops control reply sequences from inactive contexts', () => {
		const deps = buildDeps();
		deps.isContextActive.mockReturnValue(false);
		const trace = vi.fn();
		const withTrace = createTerminalInputOrchestrator({
			...deps,
			trace,
		});

		withTrace.sendInput('ws::term', '\x1b[?1;2c');

		expect(deps.write).not.toHaveBeenCalled();
		expect(trace).toHaveBeenCalledWith(
			'ws::term',
			'frontend_input_drop_inactive_context',
			expect.objectContaining({ bytes: 7 }),
		);
	});

	it('drops printable input from unfocused terminal instances', () => {
		const deps = buildDeps();
		deps.isTerminalFocused.mockReturnValue(false);
		const trace = vi.fn();
		const orchestrator = createTerminalInputOrchestrator({
			...deps,
			trace,
		});

		orchestrator.sendInput('ws::term', 'opencode');

		expect(deps.write).not.toHaveBeenCalled();
		expect(deps.appendPendingInput).not.toHaveBeenCalled();
		expect(trace).toHaveBeenCalledWith(
			'ws::term',
			'frontend_input_drop_unfocused_terminal',
			expect.objectContaining({ bytes: 8 }),
		);
	});

	it('allows mouse wheel sequences even when context is inactive', () => {
		const deps = buildDeps();
		deps.isContextActive.mockReturnValue(false);
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', '\x1b[<65;40;28M');

		expect(deps.write).toHaveBeenCalledWith('ws', 'term', '\x1b[<65;40;28M');
	});

	it('allows mouse wheel sequences even when terminal focus state is stale', () => {
		const deps = buildDeps();
		deps.isTerminalFocused.mockReturnValue(false);
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', '\x1b[<64;40;28M');

		expect(deps.write).toHaveBeenCalledWith('ws', 'term', '\x1b[<64;40;28M');
	});

	it('allows legacy X10 mouse sequences even when terminal focus state is stale', () => {
		const deps = buildDeps();
		deps.isTerminalFocused.mockReturnValue(false);
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', '\x1b[Mabc');

		expect(deps.write).toHaveBeenCalledWith('ws', 'term', '\x1b[Mabc');
	});

	it('allows rxvt mouse sequences even when terminal focus state is stale', () => {
		const deps = buildDeps();
		deps.isTerminalFocused.mockReturnValue(false);
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', '\x1b[64;40;28M');

		expect(deps.write).toHaveBeenCalledWith('ws', 'term', '\x1b[64;40;28M');
	});
});
