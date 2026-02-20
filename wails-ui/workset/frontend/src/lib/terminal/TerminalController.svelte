<script lang="ts">
	import { onDestroy } from 'svelte';
	import '@xterm/xterm/css/xterm.css';
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
		active = true,
		// eslint-disable-next-line prefer-const
		terminalContainer = null,
		// eslint-disable-next-line prefer-const
		onStateChange = undefined,
	}: Props = $props();

	let unsubscribe: (() => void) | null = null;
	let currentWorkspaceId = '';
	let currentTerminalId = '';
	let lastSyncSnapshot: {
		workspaceId: string;
		terminalId: string;
		container: HTMLDivElement | null;
		active: boolean;
	} | null = null;

	const deriveSyncSource = (current: {
		workspaceId: string;
		terminalId: string;
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
		return 'controller.effect_replay';
	};

	const bindStore = (workspace: string, terminal: string): void => {
		unsubscribe?.();
		unsubscribe = null;
		if (!workspace || !terminal) return;
		const store = getTerminalStore(workspace, terminal);
		unsubscribe = store.subscribe((state) => {
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
			container: terminalContainer,
			active,
		};
		const source = deriveSyncSource(snapshot);
		syncTerminal({
			workspaceId,
			terminalId,
			container: terminalContainer,
			active,
			source,
		});
		lastSyncSnapshot = snapshot;
	});

	onDestroy(() => {
		if (terminalId && workspaceId) {
			detachTerminal(workspaceId, terminalId);
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
