<script lang="ts">
	interface Props {
		node?: {
			terminalId?: string;
		} | null;
		focusedPaneId?: string;
		onClosePane?: (paneId: string) => void;
	}

	const { node = null, focusedPaneId = '', onClosePane = undefined }: Props = $props();
</script>

<div data-testid="mock-terminal-layout-node">{node?.terminalId ?? 'no-terminal'}</div>
<button
	type="button"
	data-testid="mock-layout-close-pane"
	onclick={() => focusedPaneId && onClosePane?.(focusedPaneId)}
>
	close pane
</button>
