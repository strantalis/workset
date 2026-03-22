<script lang="ts">
	interface Props {
		terminalId: string;
		onTerminalClosed?: () => void;
	}

	const { terminalId, onTerminalClosed = undefined }: Props = $props();
</script>

<div data-testid="mock-terminal-pane">{terminalId}</div>
<button type="button" data-testid="mock-terminal-close" onclick={() => onTerminalClosed?.()}>
	close
</button>
