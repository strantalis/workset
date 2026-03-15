import { getCurrentWindowName } from '../windowContext';

type TerminalSocketDescriptor = {
	sessionId: string;
	socketUrl?: string;
	socketToken?: string;
};

type TerminalSocketAttachReady = {
	requestedOffset?: number;
	replayStart?: number;
	replayNext?: number;
	currentOffset?: number;
	replayRequested?: boolean;
	replayTruncated?: boolean;
	replaySkipped?: boolean;
	owner?: string;
	running?: boolean;
};

type TerminalSocketControlMessage = {
	type?: string;
	error?: string;
	ready?: TerminalSocketAttachReady;
};

type TerminalSocketClientControlRequest = {
	type: 'input' | 'resize' | 'set_owner' | 'stop';
	data?: string;
	cols?: number;
	rows?: number;
	owner?: string;
};

type TerminalSocketDependencies = {
	createWebSocket?: (url: string) => WebSocket;
	getWindowName?: () => Promise<string>;
	logDebug?: (id: string, event: string, details: Record<string, unknown>) => void;
	onReady?: (id: string, ready: TerminalSocketAttachReady) => void;
	onChunk: (id: string, nextOffset: number, chunk: Uint8Array) => void;
	onClosed?: (id: string, details: { intentional: boolean; reason: string; code: number }) => void;
	onError?: (id: string, error: string) => void;
};

type ActiveSocket = {
	socket: WebSocket;
	intentional: boolean;
	connectKey: number;
	socketURL: string;
	sessionID: string;
	socketToken: string;
	windowName: string;
	ready: boolean;
	pendingMessages: string[];
};

const SOCKET_HEADER_BYTES = 8;

const resolveSocketURL = (descriptor: TerminalSocketDescriptor): string => {
	const value = descriptor.socketUrl?.trim() ?? '';
	if (!value) {
		throw new Error('terminal socket URL missing');
	}
	return value;
};

const resolveSocketToken = (descriptor: TerminalSocketDescriptor): string => {
	const value = descriptor.socketToken?.trim() ?? '';
	if (!value) {
		throw new Error('terminal socket token missing');
	}
	return value;
};

const decodeNextOffset = (buffer: ArrayBuffer): number => {
	const view = new DataView(buffer);
	const high = view.getUint32(0);
	const low = view.getUint32(4);
	return high * 2 ** 32 + low;
};

const decodeBinaryMessage = async (
	value: ArrayBuffer | Blob,
): Promise<{ nextOffset: number; chunk: Uint8Array }> => {
	const buffer = value instanceof Blob ? await value.arrayBuffer() : value;
	if (buffer.byteLength < SOCKET_HEADER_BYTES) {
		throw new Error('terminal socket frame missing offset header');
	}
	const nextOffset = decodeNextOffset(buffer);
	return {
		nextOffset,
		chunk: new Uint8Array(buffer.slice(SOCKET_HEADER_BYTES)),
	};
};

export const createTerminalSocketStream = (deps: TerminalSocketDependencies) => {
	const activeSockets = new Map<string, ActiveSocket>();
	let nextConnectKey = 1;
	const createWebSocket = deps.createWebSocket ?? ((url: string) => new WebSocket(url));
	const getWindowName = deps.getWindowName ?? getCurrentWindowName;

	const encodeControlPayload = (message: TerminalSocketClientControlRequest): string =>
		JSON.stringify({
			protocolVersion: 2,
			...message,
		});

	const flushPendingMessages = (id: string, active: ActiveSocket): void => {
		if (!active.ready || active.socket.readyState !== WebSocket.OPEN) {
			return;
		}
		if (active.pendingMessages.length === 0) {
			return;
		}
		const queued = active.pendingMessages.splice(0);
		for (const payload of queued) {
			active.socket.send(payload);
		}
		deps.logDebug?.(id, 'socket_control_flush', {
			messages: queued.length,
		});
	};

	const sendControl = (id: string, message: TerminalSocketClientControlRequest): void => {
		const active = activeSockets.get(id);
		if (!active) {
			throw new Error('terminal socket not connected');
		}
		const owner =
			message.type === 'set_owner' ? message.owner : (message.owner ?? active.windowName);
		const payload = encodeControlPayload({
			...message,
			owner,
		});
		if (active.ready && active.socket.readyState === WebSocket.OPEN) {
			active.socket.send(payload);
			deps.logDebug?.(id, 'socket_control_send', {
				type: message.type,
			});
			return;
		}
		active.pendingMessages.push(payload);
		deps.logDebug?.(id, 'socket_control_queue', {
			type: message.type,
			queueDepth: active.pendingMessages.length,
		});
	};

	const disconnect = (id: string): void => {
		const active = activeSockets.get(id);
		if (!active) return;
		active.intentional = true;
		active.socket.close(1000, 'terminal socket reset');
	};

	const disconnectAll = (): void => {
		for (const id of Array.from(activeSockets.keys())) {
			disconnect(id);
		}
	};

	const connect = async (
		id: string,
		descriptor: TerminalSocketDescriptor,
		since: number,
	): Promise<void> => {
		const socketURL = resolveSocketURL(descriptor);
		const socketToken = resolveSocketToken(descriptor);
		const sessionID = descriptor.sessionId.trim();
		if (!sessionID) {
			throw new Error('terminal session ID missing');
		}
		const existing = activeSockets.get(id);
		if (
			existing &&
			existing.socketURL === socketURL &&
			existing.sessionID === sessionID &&
			existing.socketToken === socketToken &&
			existing.socket.readyState !== WebSocket.CLOSING &&
			existing.socket.readyState !== WebSocket.CLOSED
		) {
			deps.logDebug?.(id, 'socket_connect_skip_existing', {
				socketURL,
				sessionID,
				since,
				readyState: existing.socket.readyState,
			});
			return;
		}
		disconnect(id);
		const windowName = await getWindowName();
		const streamID = `browser-${windowName}-${Date.now()}`;
		const connectKey = nextConnectKey;
		nextConnectKey += 1;
		const socket = createWebSocket(socketURL);
		socket.binaryType = 'arraybuffer';
		const active: ActiveSocket = {
			socket,
			intentional: false,
			connectKey,
			socketURL,
			sessionID,
			socketToken,
			windowName,
			ready: false,
			pendingMessages: [],
		};
		activeSockets.set(id, active);

		await new Promise<void>((resolve, reject) => {
			let settled = false;

			const fail = (error: string): void => {
				if (!settled) {
					settled = true;
					reject(new Error(error));
					return;
				}
				deps.onError?.(id, error);
			};

			const getCurrent = (): ActiveSocket | undefined => activeSockets.get(id);
			const isCurrent = (): boolean => getCurrent()?.connectKey === connectKey;

			socket.addEventListener('open', () => {
				if (!isCurrent()) return;
				deps.logDebug?.(id, 'socket_open', {
					socketURL,
					sessionID,
					streamID,
					windowName,
					since,
				});
				socket.send(
					JSON.stringify({
						protocolVersion: 2,
						type: 'attach',
						sessionId: sessionID,
						streamId: streamID,
						clientId: windowName,
						token: socketToken,
						since,
						withBuffer: true,
					}),
				);
			});

			socket.addEventListener('message', (event) => {
				if (!isCurrent()) return;
				if (typeof event.data === 'string') {
					let message: TerminalSocketControlMessage;
					try {
						message = JSON.parse(event.data) as TerminalSocketControlMessage;
					} catch (error) {
						fail(`invalid terminal socket control frame: ${String(error)}`);
						return;
					}
					if (message.type === 'ready') {
						const current = getCurrent();
						if (current) {
							current.ready = true;
							current.pendingMessages.unshift(
								encodeControlPayload({
									type: 'set_owner',
									owner: current.windowName,
								}),
							);
							flushPendingMessages(id, current);
						}
						deps.logDebug?.(id, 'socket_ready', {
							socketURL,
							sessionID,
							streamID,
							windowName,
							since,
							...message.ready,
						});
						deps.onReady?.(id, message.ready ?? {});
						if (!settled) {
							settled = true;
							resolve();
						}
						return;
					}
					if (message.type === 'error') {
						fail(message.error?.trim() || 'terminal socket attach failed');
						return;
					}
					if (message.type === 'closed') {
						deps.logDebug?.(id, 'socket_server_closed', {
							socketURL,
							sessionID,
							streamID,
						});
					}
					return;
				}

				void decodeBinaryMessage(event.data as ArrayBuffer | Blob)
					.then(({ nextOffset, chunk }) => {
						if (!isCurrent()) return;
						deps.onChunk(id, nextOffset, chunk);
					})
					.catch((error) => {
						fail(String(error));
					});
			});

			socket.addEventListener('error', () => {
				fail('terminal socket connection failed');
			});

			socket.addEventListener('close', (event) => {
				const current = getCurrent();
				const intentional = current?.intentional ?? false;
				if (current?.connectKey === connectKey) {
					activeSockets.delete(id);
				}
				if (!settled && !intentional) {
					settled = true;
					reject(new Error(event.reason || `terminal socket closed (${event.code})`));
					return;
				}
				deps.onClosed?.(id, {
					intentional,
					reason: event.reason || 'closed',
					code: event.code,
				});
			});
		});
	};

	return {
		connect,
		hasLiveConnection: (id: string): boolean => {
			const active = activeSockets.get(id);
			return Boolean(active && active.ready && active.socket.readyState === WebSocket.OPEN);
		},
		write: (id: string, data: string): void => {
			if (!data) return;
			sendControl(id, {
				type: 'input',
				data,
			});
		},
		resize: (id: string, cols: number, rows: number): void => {
			sendControl(id, {
				type: 'resize',
				cols,
				rows,
			});
		},
		stop: (id: string): void => {
			const active = activeSockets.get(id);
			if (!active) {
				throw new Error('terminal socket not connected');
			}
			active.intentional = true;
			sendControl(id, {
				type: 'stop',
			});
		},
		disconnect,
		disconnectAll,
	};
};
