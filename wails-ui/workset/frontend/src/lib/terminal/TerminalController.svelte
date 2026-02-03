<script lang="ts">
	import { onDestroy } from 'svelte';
	import '@xterm/xterm/css/xterm.css';
	import {
		detachTerminal,
		focusTerminalInstance,
		getTerminalStore,
		restartTerminal as restartServiceTerminal,
		retryHealthCheck as retryServiceHealthCheck,
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
		workspaceName,
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
			detachTerminal(currentWorkspaceId, currentTerminalId);
		}
		currentWorkspaceId = workspaceId;
		currentTerminalId = terminalId;
		bindStore(workspaceId, terminalId);
	});

	$effect(() => {
		if (!terminalId) return;
		syncTerminal({
			workspaceId,
			workspaceName,
			terminalId,
			container: terminalContainer,
			active,
		});
	});

	onDestroy(() => {
		if (terminalId && workspaceId) {
			detachTerminal(workspaceId, terminalId);
		}
		unsubscribe?.();
	});

	export async function restart(): Promise<void> {
		if (!terminalId || !workspaceId) return;
		await restartServiceTerminal(workspaceId, terminalId);
	}

	export function retryHealthCheck(): void {
		if (!terminalId || !workspaceId) return;
		retryServiceHealthCheck(workspaceId, terminalId);
	}

	export function focus(): void {
		if (!terminalId || !workspaceId) return;
		focusTerminalInstance(workspaceId, terminalId);
	}
</script>
