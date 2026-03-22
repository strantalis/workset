import { getCurrentWindowName } from '../windowContext';
import type { TerminalSnapshotLike } from './terminalEmulatorContracts';

type TerminalSocketDescriptor = {
	sessionId: string;
	socketUrl?: string;
	socketToken?: string;
	cols?: number;
	rows?: number;
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
	requestId?: string;
	ready?: TerminalSocketAttachReady;
	snapshot?: TerminalSnapshotLike;
};

type TerminalSocketClientControlRequest = {
	type: 'input' | 'resize' | 'set_owner' | 'stop' | 'snapshot';
	data?: string;
	cols?: number;
	rows?: number;
	owner?: string;
	requestId?: string;
	snapshot?: TerminalSnapshotLike;
};

type SnapshotAckWaiter = {
	resolve: () => void;
	reject: (error: Error) => void;
};

type TerminalSocketDependencies = {
	createWebSocket?: (url: string) => WebSocket;
	getWindowName?: () => Promise<string>;
	setTimeoutFn?: (callback: () => void, timeoutMs: number) => ReturnType<typeof setTimeout>;
	clearTimeoutFn?: (handle: ReturnType<typeof setTimeout>) => void;
	logDebug?: (id: string, event: string, details: Record<string, unknown>) => void;
	onReady?: (id: string, ready: TerminalSocketAttachReady) => void;
	onSnapshot?: (
		id: string,
		snapshot: TerminalSnapshotLike,
		ready: TerminalSocketAttachReady,
	) => Promise<void> | void;
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
			windowName: string;
			socketURL: string;
			since: number;
			ready: boolean;
			canWrite: boolean;
			readyMeta: TerminalSocketAttachReady;
			pendingMessages: number;
			pendingSnapshotAcks: number;
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
	windowName: string;
	ready: boolean;
	readyMeta: TerminalSocketAttachReady;
	canWrite: boolean;
	serverClosed: boolean;
	pendingMessages: string[];
	deliveryQueue: Array<() => Promise<void> | void>;
	deliveryDraining: boolean;
	pendingSnapshotAcks: Map<string, SnapshotAckWaiter>;
};

const SOCKET_HEADER_BYTES = 8;
const SNAPSHOT_ACK_TIMEOUT_MS = 750;

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
	let nextRequestID = 1;
	const createWebSocket = deps.createWebSocket ?? ((url: string) => new WebSocket(url));
	const getWindowName = deps.getWindowName ?? getCurrentWindowName;
	const setTimeoutFn =
		deps.setTimeoutFn ?? ((callback, timeoutMs) => setTimeout(callback, timeoutMs));
	const clearTimeoutFn = deps.clearTimeoutFn ?? ((handle) => clearTimeout(handle));

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
		if (message.type === 'set_owner') {
			active.canWrite = !owner || owner === active.windowName;
		}
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

		deps.logDebug?.(id, 'socket_connect_begin', {
			socketURL,
			sessionID,
			since,
			hasExisting: Boolean(existing),
			existingReadyState: existing?.socket.readyState,
			existingReady: existing?.ready,
			existingCanWrite: existing?.canWrite,
			cols: descriptor.cols,
			rows: descriptor.rows,
		});

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
			readyMeta: {},
			canWrite: false,
			serverClosed: false,
			pendingMessages: [],
			deliveryQueue: [],
			deliveryDraining: false,
			pendingSnapshotAcks: new Map(),
		};
		activeSockets.set(id, active);
		deps.logDebug?.(id, 'socket_connect_created', {
			socketURL,
			sessionID,
			streamID,
			windowName,
			since,
			connectKey,
			cols: descriptor.cols,
			rows: descriptor.rows,
		});

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
					windowName,
					since,
					cols: descriptor.cols,
					rows: descriptor.rows,
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
						cols: descriptor.cols,
						rows: descriptor.rows,
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
							current.readyMeta = message.ready ?? {};
							current.canWrite =
								!current.readyMeta.owner || current.readyMeta.owner === current.windowName;
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
					if (message.type === 'snapshot' && message.snapshot) {
						queueDelivery(async () => {
							await deps.onSnapshot?.(
								id,
								message.snapshot as TerminalSnapshotLike,
								getCurrent()?.readyMeta ?? {},
							);
						});
						return;
					}
					if (message.type === 'snapshot_ack') {
						const current = getCurrent();
						if (!current || !message.requestId) {
							return;
						}
						const waiter = current.pendingSnapshotAcks.get(message.requestId);
						if (!waiter) {
							return;
						}
						current.pendingSnapshotAcks.delete(message.requestId);
						waiter.resolve();
						return;
					}
					if (message.type === 'error') {
						const current = getCurrent();
						if (current) {
							for (const waiter of current.pendingSnapshotAcks.values()) {
								waiter.reject(new Error(message.error?.trim() || 'terminal socket request failed'));
							}
							current.pendingSnapshotAcks.clear();
						}
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
					reason: event.reason || 'closed',
					code: event.code,
					sessionID,
					streamID,
					windowName,
					socketURL,
					since,
					ready: current?.ready ?? false,
					canWrite: current?.canWrite ?? false,
					readyMeta: current?.readyMeta ?? {},
					pendingMessages: current?.pendingMessages.length ?? 0,
					pendingSnapshotAcks: current?.pendingSnapshotAcks.size ?? 0,
				};
				deps.logDebug?.(id, 'socket_close', closeDetails);
				if (current?.connectKey === connectKey) {
					for (const waiter of current.pendingSnapshotAcks.values()) {
						waiter.reject(new Error(event.reason || `terminal socket closed (${event.code})`));
					}
					current.pendingSnapshotAcks.clear();
				}
				if (current?.connectKey === connectKey) {
					activeSockets.delete(id);
				}
				if (!settled && !intentional) {
					settled = true;
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
		canWrite: (id: string): boolean => {
			const active = activeSockets.get(id);
			return Boolean(active?.canWrite);
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
		publishSnapshot: async (
			id: string,
			snapshot: TerminalSnapshotLike,
			awaitAck = false,
		): Promise<void> => {
			if (!awaitAck) {
				sendControl(id, {
					type: 'snapshot',
					snapshot,
				});
				return;
			}
			const active = activeSockets.get(id);
			if (!active) {
				throw new Error('terminal socket not connected');
			}
			const requestId = `snapshot-${nextRequestID}`;
			nextRequestID += 1;
			await new Promise<void>((resolve, reject) => {
				const timeout = setTimeoutFn(() => {
					active.pendingSnapshotAcks.delete(requestId);
					reject(new Error('terminal snapshot acknowledgement timed out'));
				}, SNAPSHOT_ACK_TIMEOUT_MS);
				const finish = (fn: () => void): void => {
					clearTimeoutFn(timeout);
					fn();
				};
				active.pendingSnapshotAcks.set(requestId, {
					resolve: () => finish(resolve),
					reject: (error) => finish(() => reject(error)),
				});
				try {
					sendControl(id, {
						type: 'snapshot',
						requestId,
						snapshot,
					});
				} catch (error) {
					clearTimeoutFn(timeout);
					active.pendingSnapshotAcks.delete(requestId);
					reject(error instanceof Error ? error : new Error(String(error)));
				}
			});
		},
		disconnect,
		disconnectAll,
	};
};
