<script lang="ts">
	import { onDestroy } from 'svelte';
	import {
		recordMockTerminalControllerDestroy,
		recordMockTerminalControllerMount,
	} from './mockTerminalControllerTracker';

	interface Props {
		workspaceId: string;
		terminalId: string;
	}

	const props: Props = $props();
	const getEntry = () => ({ workspaceId: props.workspaceId, terminalId: props.terminalId });

	recordMockTerminalControllerMount(getEntry());

	onDestroy(() => {
		recordMockTerminalControllerDestroy(getEntry());
	});

	export function focus(): void {}
	export function scrollToBottom(): void {}
	export function checkAtBottom(): boolean {
		return true;
	}
</script>
