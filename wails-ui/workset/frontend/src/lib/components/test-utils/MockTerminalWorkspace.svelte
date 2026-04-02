<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import {
		recordTerminalWorkspaceDestroy,
		recordTerminalWorkspaceMount,
	} from './mockTerminalWorkspaceTracker';

	interface Props {
		workspaceId: string;
		workspaceName: string;
	}

	const { workspaceId, workspaceName }: Props = $props();

	onMount(() => {
		recordTerminalWorkspaceMount({ workspaceId, workspaceName });
	});

	onDestroy(() => {
		recordTerminalWorkspaceDestroy({ workspaceId, workspaceName });
	});
</script>

<div data-testid="mock-terminal-workspace">{workspaceId}:{workspaceName}</div>
