import { describe, expect, it, vi } from 'vitest';
import { createTerminalInputOrchestrator } from './terminalInputOrchestrator';

const buildDeps = () => ({
	shouldSuppressMouseInput: vi.fn(() => false),
	getMode: vi.fn(() => ({ mouse: false })),
	filterMouseReports: vi.fn((_data: string, _mode: { mouse: boolean }, _tail: string) => ({
		filtered: 'input',
		tail: '',
	})),
	getMouseTail: vi.fn(() => ''),
	setMouseTail: vi.fn(),
	ensureSessionActive: vi.fn(async () => undefined),
	hasStarted: vi.fn(() => true),
	appendPendingInput: vi.fn(),
	recordOutputBytes: vi.fn(),
	getWorkspaceId: vi.fn(() => 'ws'),
	getTerminalId: vi.fn(() => 'term'),
	write: vi.fn(async () => undefined),
	markStopped: vi.fn(),
	resetTerminalInstance: vi.fn(),
	beginTerminal: vi.fn(async () => undefined),
	writeFailureMessage: vi.fn(),
});

describe('terminalInputOrchestrator', () => {
	it('queues filtered input when terminal has not started', () => {
		const deps = buildDeps();
		deps.hasStarted.mockReturnValue(false);
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', 'input');

		expect(deps.ensureSessionActive).toHaveBeenCalledWith('ws::term');
		expect(deps.appendPendingInput).toHaveBeenCalledWith('ws::term', 'input');
		expect(deps.write).not.toHaveBeenCalled();
		expect(deps.recordOutputBytes).not.toHaveBeenCalled();
	});

	it('writes filtered input when terminal is started', () => {
		const deps = buildDeps();
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', 'input');

		expect(deps.recordOutputBytes).toHaveBeenCalledWith('ws::term', 5);
		expect(deps.write).toHaveBeenCalledWith('ws', 'term', 'input');
	});

	it('recovers session when write fails with a recoverable string error', async () => {
		const deps = buildDeps();
		deps.write.mockRejectedValue('session not found');
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', 'input');
		await Promise.resolve();
		await Promise.resolve();

		expect(deps.appendPendingInput).toHaveBeenCalledWith('ws::term', 'input');
		expect(deps.markStopped).toHaveBeenCalledWith('ws::term');
		expect(deps.resetTerminalInstance).toHaveBeenCalledWith('ws::term');
		expect(deps.beginTerminal).toHaveBeenCalledWith('ws::term', true);
		expect(deps.writeFailureMessage).toHaveBeenCalledWith('ws::term', 'session not found');
	});

	it('does not recover session for non-recoverable errors', async () => {
		const deps = buildDeps();
		deps.write.mockRejectedValue(new Error('write refused'));
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', 'input');
		await Promise.resolve();
		await Promise.resolve();

		expect(deps.appendPendingInput).toHaveBeenCalledWith('ws::term', 'input');
		expect(deps.markStopped).toHaveBeenCalledWith('ws::term');
		expect(deps.resetTerminalInstance).not.toHaveBeenCalled();
		expect(deps.beginTerminal).not.toHaveBeenCalled();
		expect(deps.writeFailureMessage).toHaveBeenCalledWith('ws::term', 'Error: write refused');
	});

	it('short-circuits when mouse input is suppressed', () => {
		const deps = buildDeps();
		deps.shouldSuppressMouseInput.mockReturnValue(true);
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', 'input');

		expect(deps.filterMouseReports).not.toHaveBeenCalled();
		expect(deps.ensureSessionActive).not.toHaveBeenCalled();
		expect(deps.write).not.toHaveBeenCalled();
	});

	it('updates mouse tail even when filtered input is empty', () => {
		const deps = buildDeps();
		deps.getMouseTail.mockReturnValue('old-tail');
		deps.filterMouseReports.mockReturnValue({ filtered: '', tail: 'new-tail' });
		const orchestrator = createTerminalInputOrchestrator(deps);

		orchestrator.sendInput('ws::term', 'partial');

		expect(deps.setMouseTail).toHaveBeenCalledWith('ws::term', 'new-tail');
		expect(deps.ensureSessionActive).not.toHaveBeenCalled();
		expect(deps.appendPendingInput).not.toHaveBeenCalled();
	});
});
