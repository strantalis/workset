import type {
	FitAddonLike,
	TerminalLike,
	TerminalLinkProviderLike,
} from './terminalEmulatorContracts';

type TerminalInstanceHandle = {
	terminal: TerminalLike;
	fitAddon: FitAddonLike;
	linkProviders?: TerminalLinkProviderLike[];
	linkProvidersRegistered?: boolean;
	opened?: boolean;
	openWindow?: Window | null;
	dataDisposable: {
		dispose: () => void;
	};
	container: HTMLDivElement;
};

export type { TerminalInstanceHandle };

type TerminalInstanceManagerDeps = {
	terminalHandles: Map<string, TerminalInstanceHandle>;
	createTerminalInstance: () => Promise<unknown>;
	createFitAddon: () => FitAddonLike;
	createLinkProviders?: (terminal: TerminalLike) => TerminalLinkProviderLike[];
	createHostContainer?: () => HTMLDivElement;
	onData: (id: string, data: string) => void;
	onResponse?: (id: string, data: string) => void;
	onRendererError?: (id: string, message: string) => void;
	onRendererDebug?: (id: string, event: string, details: Record<string, unknown>) => void;
	attachOpen: (input: {
		id: string;
		handle: TerminalInstanceHandle;
		container: HTMLDivElement | null;
		active: boolean;
	}) => void | Promise<void>;
};

type DataDisposables = {
	dataDisposable: {
		dispose: () => void;
	};
	responseDisposable: {
		dispose: () => void;
	};
};

const createDefaultHostContainer = (): HTMLDivElement => {
	const host = document.createElement('div');
	host.className = 'terminal-instance';
	return host;
};

const subscribeDataEvents = (
	id: string,
	terminal: TerminalLike,
	consumeData: (id: string, data: string) => void,
	consumeResponse: ((id: string, data: string) => void) | undefined,
): DataDisposables => {
	const onDataDisposable = terminal.onData((data) => {
		consumeData(id, data);
	});
	const onResponseDisposable = terminal.onResponse?.((data) => {
		consumeResponse?.(id, data);
	}) ?? { dispose: () => undefined };

	return {
		dataDisposable: {
			dispose: () => {
				onDataDisposable.dispose();
			},
		},
		responseDisposable: {
			dispose: () => {
				onResponseDisposable.dispose();
			},
		},
	};
};

const registerLinkProviders = (
	id: string,
	deps: TerminalInstanceManagerDeps,
	handle: TerminalInstanceHandle,
): void => {
	if (handle.linkProvidersRegistered) return;
	const providers = handle.linkProviders ?? [];
	handle.linkProvidersRegistered = true;
	if (providers.length === 0) {
		return;
	}
	const register = handle.terminal.registerLinkProvider;
	if (!register) {
		deps.onRendererError?.(id, 'Terminal link provider API unavailable');
		return;
	}
	try {
		for (const provider of providers) {
			register.call(handle.terminal, provider);
		}
		deps.onRendererDebug?.(id, 'link_providers_registered', { count: providers.length });
	} catch (error) {
		const message = error instanceof Error ? error.message : 'Failed to register link providers';
		deps.onRendererError?.(id, message);
		deps.onRendererDebug?.(id, 'link_providers_registration_error', { message });
	}
};

export const createTerminalInstanceManager = (deps: TerminalInstanceManagerDeps) => {
	const creatingTerminalPromises = new Map<string, Promise<TerminalInstanceHandle>>();

	const disposeHandle = (id: string, handle: TerminalInstanceHandle): void => {
		creatingTerminalPromises.delete(id);
		handle.dataDisposable?.dispose();
		for (const provider of handle.linkProviders ?? []) {
			provider.dispose?.();
		}
		handle.terminal.dispose();
		if (deps.terminalHandles.get(id) === handle) {
			deps.terminalHandles.delete(id);
		}
	};

	const createHandle = async (
		id: string,
		container: HTMLDivElement | null,
		active: boolean,
	): Promise<TerminalInstanceHandle> => {
		const terminal = (await deps.createTerminalInstance()) as TerminalLike;
		terminal.options.cursorBlink = Boolean(active && terminal.options.cursorBlink);
		const fitAddon = deps.createFitAddon();
		if (terminal.loadAddon) {
			terminal.loadAddon(fitAddon);
		}
		const { dataDisposable, responseDisposable } = subscribeDataEvents(
			id,
			terminal,
			deps.onData,
			deps.onResponse,
		);
		const createHost = deps.createHostContainer ?? createDefaultHostContainer;
		const handle: TerminalInstanceHandle = {
			terminal,
			fitAddon,
			linkProviders: deps.createLinkProviders?.(terminal) ?? [],
			linkProvidersRegistered: false,
			dataDisposable,
			container: createHost(),
		};
		handle.dataDisposable = {
			dispose: () => {
				dataDisposable.dispose();
				responseDisposable.dispose();
			},
		};
		deps.onRendererDebug?.(id, 'terminal_instance_created', {
			hasContainer: Boolean(container),
			active,
		});
		return handle;
	};

	return {
		attach: async (id: string, container: HTMLDivElement | null, active: boolean) => {
			let handle = deps.terminalHandles.get(id);
			if (!handle) {
				let pending = creatingTerminalPromises.get(id);
				if (!pending) {
					pending = (async () => {
						const created = await createHandle(id, container, active);
						deps.terminalHandles.set(id, created);
						return created;
					})();
					creatingTerminalPromises.set(id, pending);
					pending.finally(() => {
						creatingTerminalPromises.delete(id);
					});
				}
				handle = await pending;
			}
			deps.onRendererDebug?.(id, 'terminal_attach_open_request', {
				active,
				hasContainer: Boolean(container),
			});
			try {
				await deps.attachOpen({ id, handle, container, active });
			} catch (error) {
				const message =
					error instanceof Error ? error.message : 'Failed to attach/open terminal instance';
				deps.onRendererDebug?.(id, 'terminal_attach_open_failed', {
					active,
					hasContainer: Boolean(container),
					message,
				});
				disposeHandle(id, handle);
				throw error;
			}
			registerLinkProviders(id, deps, handle);
			return handle;
		},

		dispose: (id: string): void => {
			const handle = deps.terminalHandles.get(id);
			if (!handle) return;
			disposeHandle(id, handle);
			deps.onRendererDebug?.(id, 'terminal_instance_disposed', {});
		},
	};
};
