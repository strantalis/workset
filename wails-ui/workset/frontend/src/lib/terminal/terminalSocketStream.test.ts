import { afterEach, describe, expect, it, vi } from 'vitest';
import { createTerminalSocketStream } from './terminalSocketStream';

class MockWebSocket {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	public binaryType = 'blob';
	public readyState = MockWebSocket.CONNECTING;
	public readonly sent: unknown[] = [];
	private listeners = new Map<string, Set<(event: unknown) => void>>();

	constructor(public readonly url: string) {}

	addEventListener(type: string, listener: (event: unknown) => void): void {
		const current = this.listeners.get(type) ?? new Set();
		current.add(listener);
		this.listeners.set(type, current);
	}

	send(data: unknown): void {
		this.sent.push(data);
	}

	close(code = 1000, reason = ''): void {
		this.readyState = MockWebSocket.CLOSED;
		this.dispatch('close', { code, reason, wasClean: true });
	}

	open(): void {
		this.readyState = MockWebSocket.OPEN;
		this.dispatch('open', {});
	}

	emitText(data: unknown): void {
		this.dispatch('message', { data: JSON.stringify(data) });
	}

	emitBinary(payload: Uint8Array): void {
		this.dispatch('message', { data: payload.buffer.slice(0) });
	}

	private dispatch(type: string, event: unknown): void {
		for (const listener of this.listeners.get(type) ?? []) {
			listener(event);
		}
	}
}

const createDescriptor = () => ({
	workspaceId: 'ws',
	terminalId: 'term',
	sessionId: 'ws::term',
	windowName: 'main',
	owner: 'main',
	canWrite: true,
	running: true,
	currentOffset: 0,
	socketUrl: 'ws://127.0.0.1:9001/stream',
	socketToken: 'token',
	transport: 'sessiond-websocket',
});

const encodeChunk = (seq: number, value: string): Uint8Array => {
	const text = new TextEncoder().encode(value);
	const payload = new Uint8Array(8 + text.length);
	const view = new DataView(payload.buffer);
	view.setBigUint64(0, BigInt(seq), false);
	payload.set(text, 8);
	return payload;
};

const createSnapshot = () => ({
	version: 1,
	nextOffset: 22,
	cols: 80,
	rows: 24,
	activeBuffer: 'normal' as const,
	normalViewportY: 0,
	cursor: { x: 0, y: 0, visible: true },
	modes: { dec: [], ansi: [] },
	normalTail: ['hello'],
	normalScreen: ['hello'],
});

const waitForExpectation = async (check: () => void, attempts = 20): Promise<void> => {
	let lastError: unknown;
	for (let attempt = 0; attempt < attempts; attempt += 1) {
		try {
			check();
			return;
		} catch (error) {
			lastError = error;
			await Promise.resolve();
		}
	}
	throw lastError instanceof Error ? lastError : new Error(String(lastError));
};

describe('terminalSocketStream', () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	it('sends an attach request with resume offset and token', async () => {
		const socket = new MockWebSocket('ws://127.0.0.1:9001/stream');
		const onChunk = vi.fn();
		const stream = createTerminalSocketStream({
			createWebSocket: () => socket as unknown as WebSocket,
			getWindowName: async () => 'main',
			onChunk,
		});

		const connectPromise = stream.connect('ws::term', createDescriptor(), 14);
		await Promise.resolve();
		socket.open();
		socket.emitText({ type: 'ready', ready: { running: true } });

		expect(socket.sent).toHaveLength(2);
		expect(JSON.parse(String(socket.sent[0]))).toEqual({
			protocolVersion: 2,
			type: 'attach',
			sessionId: 'ws::term',
			streamId: expect.any(String),
			clientId: 'main',
			token: 'token',
			since: 14,
			withBuffer: true,
		});
		expect(JSON.parse(String(socket.sent[1]))).toEqual({
			protocolVersion: 2,
			type: 'set_owner',
			owner: 'main',
		});
		expect(onChunk).not.toHaveBeenCalled();
		await connectPromise;
	});

	it('decodes binary stream chunks and forwards seq plus bytes', async () => {
		const socket = new MockWebSocket('ws://127.0.0.1:9001/stream');
		const onChunk = vi.fn();
		const stream = createTerminalSocketStream({
			createWebSocket: () => socket as unknown as WebSocket,
			getWindowName: async () => 'main',
			onChunk,
		});

		const connectPromise = stream.connect('ws::term', createDescriptor(), 0);
		await Promise.resolve();
		socket.open();
		socket.emitText({ type: 'ready', ready: { running: true } });
		await connectPromise;
		socket.emitBinary(encodeChunk(22, 'hello'));

		await waitForExpectation(() => {
			expect(onChunk).toHaveBeenCalledTimes(1);
		});
		expect(onChunk.mock.calls[0]?.[0]).toBe('ws::term');
		expect(onChunk.mock.calls[0]?.[1]).toBe(22);
		expect(Array.from(onChunk.mock.calls[0]?.[2] as Uint8Array)).toEqual(
			Array.from(new TextEncoder().encode('hello')),
		);
	});

	it('delivers snapshot hydration before subsequent chunks', async () => {
		const socket = new MockWebSocket('ws://127.0.0.1:9001/stream');
		const onChunk = vi.fn();
		const order: string[] = [];
		let releaseSnapshot: (() => void) | undefined;
		const snapshotPromise = new Promise<void>((resolve) => {
			releaseSnapshot = resolve;
		});
		const stream = createTerminalSocketStream({
			createWebSocket: () => socket as unknown as WebSocket,
			getWindowName: async () => 'main',
			onChunk: (...args) => {
				order.push('chunk');
				onChunk(...args);
			},
			onSnapshot: async () => {
				order.push('snapshot:start');
				await snapshotPromise;
				order.push('snapshot:end');
			},
		});

		const connectPromise = stream.connect('ws::term', createDescriptor(), 0);
		await Promise.resolve();
		socket.open();
		socket.emitText({ type: 'ready', ready: { running: true } });
		await connectPromise;

		socket.emitText({ type: 'snapshot', snapshot: createSnapshot() });
		socket.emitBinary(encodeChunk(30, 'delta'));
		await Promise.resolve();

		expect(order).toEqual(['snapshot:start']);
		expect(onChunk).not.toHaveBeenCalled();

		releaseSnapshot?.();

		await waitForExpectation(() => {
			expect(order).toEqual(['snapshot:start', 'snapshot:end', 'chunk']);
			expect(onChunk).toHaveBeenCalledTimes(1);
		});
	});

	it('awaits snapshot publish acknowledgement when requested', async () => {
		const socket = new MockWebSocket('ws://127.0.0.1:9001/stream');
		const stream = createTerminalSocketStream({
			createWebSocket: () => socket as unknown as WebSocket,
			getWindowName: async () => 'main',
			onChunk: vi.fn(),
		});

		const connectPromise = stream.connect('ws::term', createDescriptor(), 0);
		await Promise.resolve();
		socket.open();
		socket.emitText({ type: 'ready', ready: { running: true } });
		await connectPromise;

		const publishPromise = stream.publishSnapshot('ws::term', createSnapshot(), true);
		const payload = JSON.parse(String(socket.sent.at(-1)));
		expect(payload).toMatchObject({
			protocolVersion: 2,
			type: 'snapshot',
			owner: 'main',
			requestId: expect.any(String),
		});

		let settled = false;
		void publishPromise.then(() => {
			settled = true;
		});
		await Promise.resolve();
		expect(settled).toBe(false);

		socket.emitText({ type: 'snapshot_ack', requestId: payload.requestId });
		await publishPromise;
		expect(settled).toBe(true);
	});

	it('times out snapshot acknowledgement waits so callers do not hang forever', async () => {
		vi.useFakeTimers();
		const socket = new MockWebSocket('ws://127.0.0.1:9001/stream');
		const stream = createTerminalSocketStream({
			createWebSocket: () => socket as unknown as WebSocket,
			getWindowName: async () => 'main',
			onChunk: vi.fn(),
			setTimeoutFn: (callback, timeoutMs) => setTimeout(callback, timeoutMs),
			clearTimeoutFn: (handle) => clearTimeout(handle),
		});

		const connectPromise = stream.connect('ws::term', createDescriptor(), 0);
		await Promise.resolve();
		socket.open();
		socket.emitText({ type: 'ready', ready: { running: true } });
		await connectPromise;

		const publishPromise = stream.publishSnapshot('ws::term', createSnapshot(), true);
		const publishAssertion = expect(publishPromise).rejects.toThrow(
			'terminal snapshot acknowledgement timed out',
		);
		await vi.advanceTimersByTimeAsync(751);

		await publishAssertion;
	});

	it('rejects connect on remote attach error', async () => {
		const socket = new MockWebSocket('ws://127.0.0.1:9001/stream');
		const onClosed = vi.fn();
		const stream = createTerminalSocketStream({
			createWebSocket: () => socket as unknown as WebSocket,
			getWindowName: async () => 'main',
			onChunk: vi.fn(),
			onClosed,
		});

		const connectPromise = stream.connect('ws::term', createDescriptor(), 0);
		await Promise.resolve();
		socket.open();
		socket.emitText({ type: 'error', error: 'invalid websocket token' });
		await expect(connectPromise).rejects.toThrow('invalid websocket token');
		expect(onClosed).not.toHaveBeenCalled();
	});

	it('reports local resets as intentional closes', async () => {
		const sockets: MockWebSocket[] = [];
		const onClosed = vi.fn();
		const stream = createTerminalSocketStream({
			createWebSocket: (url) => {
				const socket = new MockWebSocket(url);
				sockets.push(socket);
				return socket as unknown as WebSocket;
			},
			getWindowName: async () => 'main',
			onChunk: vi.fn(),
			onClosed,
		});

		const firstConnect = stream.connect('ws::term', createDescriptor(), 0);
		await Promise.resolve();
		sockets[0].open();
		sockets[0].emitText({ type: 'ready', ready: { running: true } });
		await firstConnect;

		const nextDescriptor = {
			...createDescriptor(),
			socketToken: 'token-2',
		};
		const secondConnect = stream.connect('ws::term', nextDescriptor, 0);
		await Promise.resolve();

		expect(onClosed).toHaveBeenCalledWith(
			'ws::term',
			expect.objectContaining({
				intentional: true,
				reason: 'terminal socket reset',
				code: 1000,
				sessionID: 'ws::term',
				windowName: 'main',
				socketURL: 'ws://127.0.0.1:9001/stream',
				ready: true,
				canWrite: true,
			}),
		);

		sockets[1].open();
		sockets[1].emitText({ type: 'ready', ready: { running: true } });
		await secondConnect;
	});

	it('skips reconnecting when the same session socket is already live', async () => {
		const sockets: MockWebSocket[] = [];
		const logDebug = vi.fn();
		const stream = createTerminalSocketStream({
			createWebSocket: (url) => {
				const socket = new MockWebSocket(url);
				sockets.push(socket);
				return socket as unknown as WebSocket;
			},
			getWindowName: async () => 'main',
			onChunk: vi.fn(),
			logDebug,
		});

		const descriptor = createDescriptor();
		const firstConnect = stream.connect('ws::term', descriptor, 0);
		await Promise.resolve();
		sockets[0].open();
		sockets[0].emitText({ type: 'ready', ready: { running: true } });
		await firstConnect;

		await stream.connect('ws::term', descriptor, 128);

		expect(sockets).toHaveLength(1);
		expect(logDebug).toHaveBeenCalledWith(
			'ws::term',
			'socket_connect_skip_existing',
			expect.objectContaining({
				sessionID: 'ws::term',
				since: 128,
			}),
		);
	});

	it('sends input, resize, snapshot, and stop over the live websocket', async () => {
		const socket = new MockWebSocket('ws://127.0.0.1:9001/stream');
		const stream = createTerminalSocketStream({
			createWebSocket: () => socket as unknown as WebSocket,
			getWindowName: async () => 'main',
			onChunk: vi.fn(),
		});

		const connectPromise = stream.connect('ws::term', createDescriptor(), 0);
		await Promise.resolve();
		socket.open();
		socket.emitText({ type: 'ready', ready: { running: true } });
		await connectPromise;

		stream.write('ws::term', 'ls\n');
		stream.resize('ws::term', 120, 32);
		stream.publishSnapshot('ws::term', createSnapshot());
		stream.stop('ws::term');

		expect(JSON.parse(String(socket.sent[2]))).toEqual({
			protocolVersion: 2,
			type: 'input',
			data: 'ls\n',
			owner: 'main',
		});
		expect(JSON.parse(String(socket.sent[3]))).toEqual({
			protocolVersion: 2,
			type: 'resize',
			cols: 120,
			rows: 32,
			owner: 'main',
		});
		expect(JSON.parse(String(socket.sent[4]))).toEqual({
			protocolVersion: 2,
			type: 'snapshot',
			snapshot: createSnapshot(),
			owner: 'main',
		});
		expect(JSON.parse(String(socket.sent[5]))).toEqual({
			protocolVersion: 2,
			type: 'stop',
			owner: 'main',
		});
	});
});
