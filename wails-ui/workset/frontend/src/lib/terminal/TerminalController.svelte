<script lang="ts">
	import { onDestroy } from 'svelte';
	import { logTerminalDebug } from '../api/terminal-layout';
	import type { TerminalSnapshotLike } from './terminalEmulatorContracts';
	import {
		detachTerminal,
		focusTerminalInstance,
		getTerminalStore,
		scrollTerminalToBottom,
		isTerminalAtBottom,
		syncTerminal,
		type TerminalViewState,
	} from './terminalService';

	interface Props {
		workspaceId: string;
		workspaceName: string;
		terminalId: string;
		initialSnapshot?: TerminalSnapshotLike | null;
		active?: boolean;
		terminalContainer?: HTMLDivElement | null;
		onStateChange?: (state: TerminalViewState) => void;
	}

	// Props must use 'let' for Svelte 5 reactivity ($props() pattern)
	let {
		// eslint-disable-next-line prefer-const
		workspaceId,
		// eslint-disable-next-line prefer-const
		terminalId,
		// eslint-disable-next-line prefer-const
		initialSnapshot = null,
		// eslint-disable-next-line prefer-const
		active = true,
		// eslint-disable-next-line prefer-const
		terminalContainer = null,
		// eslint-disable-next-line prefer-const
		onStateChange = undefined,
	}: Props = $props();

	let unsubscribe: (() => void) | null = null;
	let currentWorkspaceId = '';
	let currentTerminalId = '';
	let lastLoggedStateSignature = '';
	let lastSyncSnapshot: {
		workspaceId: string;
		terminalId: string;
		initialSnapshot: TerminalSnapshotLike | null;
		container: HTMLDivElement | null;
		active: boolean;
	} | null = null;

	const deriveSyncSource = (current: {
		workspaceId: string;
		terminalId: string;
		initialSnapshot: TerminalSnapshotLike | null;
		container: HTMLDivElement | null;
		active: boolean;
	}): string => {
		const previous = lastSyncSnapshot;
		if (!previous) return 'controller.initial';
		if (
			previous.workspaceId !== current.workspaceId &&
			previous.terminalId !== current.terminalId
		) {
			return 'controller.workspace_and_terminal_change';
		}
		if (previous.workspaceId !== current.workspaceId) {
			return 'controller.workspace_change';
		}
		if (previous.terminalId !== current.terminalId) {
			return 'controller.terminal_change';
		}
		if (previous.container !== current.container && previous.active !== current.active) {
			return 'controller.container_and_active_change';
		}
		if (previous.container !== current.container) {
			return 'controller.container_change';
		}
		if (previous.active !== current.active) {
			return 'controller.active_change';
		}
		return 'controller.effect_repeat';
	};

	const logControllerEvent = (event: string, details: Record<string, unknown>): void => {
		if (!workspaceId || !terminalId) return;
		void logTerminalDebug(workspaceId, terminalId, event, JSON.stringify(details));
	};

	const logControllerState = (state: TerminalViewState): void => {
		const signature = JSON.stringify({
			status: state.status,
			message: state.message,
			health: state.health,
			healthMessage: state.healthMessage,
			active,
			hasContainer: Boolean(terminalContainer),
		});
		if (signature === lastLoggedStateSignature) {
			return;
		}
		lastLoggedStateSignature = signature;
		logControllerEvent('frontend_controller_state', {
			status: state.status,
			message: state.message,
			health: state.health,
			healthMessage: state.healthMessage,
			active,
			hasContainer: Boolean(terminalContainer),
		});
	};

	const bindStore = (workspace: string, terminal: string): void => {
		unsubscribe?.();
		unsubscribe = null;
		if (!workspace || !terminal) return;
		logControllerEvent('frontend_controller_bind', {
			workspaceId: workspace,
			terminalId: terminal,
		});
		const store = getTerminalStore(workspace, terminal);
		unsubscribe = store.subscribe((state) => {
			logControllerState(state);
			onStateChange?.(state);
		});
	};

	$effect(() => {
		if (terminalId === currentTerminalId && workspaceId === currentWorkspaceId) return;
		if (currentTerminalId && currentWorkspaceId) {
			// Normal tab/pane switches reuse the same mount container; let syncTerminal
			// displace the previous terminal binding instead of forcing an eager detach.
			if (!terminalContainer || !terminalContainer.isConnected) {
				detachTerminal(currentWorkspaceId, currentTerminalId, { force: true });
			}
		}
		currentWorkspaceId = workspaceId;
		currentTerminalId = terminalId;
		bindStore(workspaceId, terminalId);
	});

	$effect(() => {
		if (!terminalId) return;
		if (!terminalContainer || !terminalContainer.isConnected) return;
		const snapshot = {
			workspaceId,
			terminalId,
			initialSnapshot,
			container: terminalContainer,
			active,
		};
		const source = deriveSyncSource(snapshot);
		logControllerEvent('frontend_controller_sync', {
			source,
			active,
			hasContainer: Boolean(terminalContainer),
			hasInitialSnapshot: Boolean(initialSnapshot),
		});
		syncTerminal({
			workspaceId,
			terminalId,
			container: terminalContainer,
			active,
			initialSnapshot,
			source,
		});
		lastSyncSnapshot = snapshot;
	});

	onDestroy(() => {
		logControllerEvent('frontend_controller_destroy', {
			workspaceId,
			terminalId,
		});
		if (terminalId && workspaceId) {
			// Destroyed controllers must release container ownership immediately so the
			// replacement controller can attach without displacing stale state.
			detachTerminal(workspaceId, terminalId, { force: true });
		}
		unsubscribe?.();
	});

	export function focus(): void {
		if (!terminalId || !workspaceId) return;
		focusTerminalInstance(workspaceId, terminalId);
	}

	export function scrollToBottom(): void {
		if (!terminalId || !workspaceId) return;
		scrollTerminalToBottom(workspaceId, terminalId);
	}

	export function checkAtBottom(): boolean {
		if (!terminalId || !workspaceId) return true;
		return isTerminalAtBottom(workspaceId, terminalId);
	}
</script>
