<script lang="ts">
	import type { TerminalSnapshotLike } from '../../terminal/terminalEmulatorContracts';

	interface Props {
		terminalId: string;
		initialSnapshot?: TerminalSnapshotLike | null;
		onTerminalClosed?: () => void;
	}

	const { terminalId, initialSnapshot = null, onTerminalClosed = undefined }: Props = $props();
</script>

<div data-testid="mock-terminal-pane" data-has-snapshot={initialSnapshot ? 'true' : 'false'}>
	{terminalId}
</div>
<button type="button" data-testid="mock-terminal-close" onclick={() => onTerminalClosed?.()}>
	close
</button>
