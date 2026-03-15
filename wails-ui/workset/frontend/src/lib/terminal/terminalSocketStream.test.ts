import { describe, expect, it, vi } from 'vitest';
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

describe('terminalSocketStream', () => {
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
		await Promise.resolve();

		expect(onChunk).toHaveBeenCalledTimes(1);
		expect(onChunk.mock.calls[0]?.[0]).toBe('ws::term');
		expect(onChunk.mock.calls[0]?.[1]).toBe(22);
		expect(Array.from(onChunk.mock.calls[0]?.[2] as Uint8Array)).toEqual(
			Array.from(new TextEncoder().encode('hello')),
		);
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

	it('sends input, resize, and stop over the live websocket', async () => {
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
			type: 'stop',
			owner: 'main',
		});
	});
});
