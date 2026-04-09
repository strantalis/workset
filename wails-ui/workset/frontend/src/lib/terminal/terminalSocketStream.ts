type TerminalSocketDescriptor = {
	sessionId: string;
	socketUrl?: string;
	socketToken?: string;
	startOffset?: number;
};

type TerminalSocketControlMessage = {
	type?: string;
	error?: string;
};

type TerminalSocketClientControlRequest = {
	type: 'input' | 'resize' | 'stop';
	data?: string;
	cols?: number;
	rows?: number;
};

type TerminalSocketDependencies = {
	createWebSocket?: (url: string) => WebSocket;
	logDebug?: (id: string, event: string, details: Record<string, unknown>) => void;
	onReady?: (id: string) => void;
	onChunk: (id: string, nextOffset: number, chunk: Uint8Array) => void;
	onClosed?: (
		id: string,
		details: {
			intentional: boolean;
			serverClosed: boolean;
			reason: string;
			code: number;
			sessionID: string;
			streamID: string;
			socketURL: string;
			ready: boolean;
			pendingMessages: number;
		},
	) => void;
	onError?: (id: string, error: string) => void;
};

type ActiveSocket = {
	socket: WebSocket;
	intentional: boolean;
	connectKey: number;
	socketURL: string;
	sessionID: string;
	socketToken: string;
	ready: boolean;
	serverClosed: boolean;
	pendingMessages: string[];
	deliveryQueue: Array<() => Promise<void> | void>;
	deliveryDraining: boolean;
};

const SOCKET_HEADER_BYTES = 8;
const SOCKET_CONNECT_TIMEOUT_MS = 10_000;

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

const decodeBinaryBuffer = (buffer: ArrayBuffer): { nextOffset: number; chunk: Uint8Array } => {
	if (buffer.byteLength < SOCKET_HEADER_BYTES) {
		throw new Error('terminal socket frame missing offset header');
	}
	const nextOffset = decodeNextOffset(buffer);
	return {
		nextOffset,
		chunk: new Uint8Array(buffer.slice(SOCKET_HEADER_BYTES)),
	};
};

const decodeBinaryMessage = async (
	value: Blob,
): Promise<{ nextOffset: number; chunk: Uint8Array }> =>
	decodeBinaryBuffer(await value.arrayBuffer());

export const createTerminalSocketStream = (deps: TerminalSocketDependencies) => {
	const activeSockets = new Map<string, ActiveSocket>();
	let nextConnectKey = 1;
	const createWebSocket = deps.createWebSocket ?? ((url: string) => new WebSocket(url));

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
		const payload = encodeControlPayload(message);
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

	const connect = async (id: string, descriptor: TerminalSocketDescriptor): Promise<void> => {
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
				readyState: existing.socket.readyState,
			});
			return;
		}

		deps.logDebug?.(id, 'socket_connect_begin', {
			socketURL,
			sessionID,
			hasExisting: Boolean(existing),
			existingReadyState: existing?.socket.readyState,
			existingReady: existing?.ready,
		});

		disconnect(id);

		const streamID = `browser-${Date.now()}-${nextConnectKey}`;
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
			ready: false,
			serverClosed: false,
			pendingMessages: [],
			deliveryQueue: [],
			deliveryDraining: false,
		};
		activeSockets.set(id, active);
		deps.logDebug?.(id, 'socket_connect_created', {
			socketURL,
			sessionID,
			streamID,
			connectKey,
		});

		await new Promise<void>((resolve, reject) => {
			let settled = false;

			const connectTimeout = setTimeout(() => {
				if (settled) return;
				settled = true;
				deps.logDebug?.(id, 'socket_connect_timeout', {
					socketURL,
					sessionID,
					streamID,
					timeoutMs: SOCKET_CONNECT_TIMEOUT_MS,
				});
				const current = activeSockets.get(id);
				if (current?.connectKey === connectKey) {
					current.intentional = true;
					current.socket.close(1000, 'connect timeout');
				}
				reject(new Error('Terminal socket connect timed out.'));
			}, SOCKET_CONNECT_TIMEOUT_MS);

			const fail = (error: string): void => {
				if (!settled) {
					settled = true;
					clearTimeout(connectTimeout);
					reject(new Error(error));
					return;
				}
				deps.onError?.(id, error);
			};

			const getCurrent = (): ActiveSocket | undefined => activeSockets.get(id);
			const isCurrent = (): boolean => getCurrent()?.connectKey === connectKey;
			const drainDeliveries = async (): Promise<void> => {
				const current = getCurrent();
				if (!current || current.deliveryDraining) return;
				current.deliveryDraining = true;
				try {
					while (current.deliveryQueue.length > 0) {
						const next = current.deliveryQueue.shift();
						if (!next) continue;
						const result = next();
						if (result && typeof (result as PromiseLike<void>).then === 'function') {
							await result;
						}
						if (!isCurrent()) {
							return;
						}
					}
				} catch (error) {
					fail(String(error));
				} finally {
					const latest = getCurrent();
					if (latest?.connectKey === connectKey) {
						latest.deliveryDraining = false;
					}
				}
			};
			const queueDelivery = (run: () => Promise<void> | void): void => {
				const current = getCurrent();
				if (!current) return;
				current.deliveryQueue.push(run);
				if (!current.deliveryDraining) {
					void drainDeliveries();
				}
			};

			socket.addEventListener('open', () => {
				if (!isCurrent()) return;
				deps.logDebug?.(id, 'socket_open', {
					socketURL,
					sessionID,
					streamID,
				});
				socket.send(
					JSON.stringify({
						protocolVersion: 2,
						type: 'attach',
						sessionId: sessionID,
						streamId: streamID,
						token: socketToken,
						startOffset: descriptor.startOffset ?? 0,
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
							flushPendingMessages(id, current);
						}
						deps.logDebug?.(id, 'socket_ready', {
							socketURL,
							sessionID,
							streamID,
						});
						deps.onReady?.(id);
						if (!settled) {
							settled = true;
							clearTimeout(connectTimeout);
							resolve();
						}
						return;
					}
					if (message.type === 'error') {
						fail(message.error?.trim() || 'terminal socket attach failed');
						return;
					}
					if (message.type === 'closed') {
						const current = getCurrent();
						if (current) {
							current.serverClosed = true;
						}
						deps.logDebug?.(id, 'socket_server_closed', {
							socketURL,
							sessionID,
							streamID,
						});
					}
					return;
				}

				if (event.data instanceof Blob) {
					queueDelivery(async () => {
						const { nextOffset, chunk } = await decodeBinaryMessage(event.data);
						if (!isCurrent()) return;
						deps.onChunk(id, nextOffset, chunk);
					});
					return;
				}

				const { nextOffset, chunk } = decodeBinaryBuffer(event.data as ArrayBuffer);
				queueDelivery(() => {
					if (!isCurrent()) return;
					deps.onChunk(id, nextOffset, chunk);
				});
			});

			socket.addEventListener('error', () => {
				fail('terminal socket connection failed');
			});

			socket.addEventListener('close', (event) => {
				const current = getCurrent();
				const intentional = current?.intentional ?? false;
				const closeDetails = {
					intentional,
					serverClosed: current?.serverClosed ?? false,
					reason: event.reason || '',
					code: event.code,
					sessionID,
					streamID,
					socketURL,
					ready: current?.ready ?? false,
					pendingMessages: current?.pendingMessages.length ?? 0,
				};
				if (current?.connectKey === connectKey) {
					activeSockets.delete(id);
				}
				if (!settled) {
					settled = true;
					clearTimeout(connectTimeout);
					reject(new Error(event.reason || `terminal socket closed (${event.code})`));
					return;
				}
				deps.onClosed?.(id, closeDetails);
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
