<script lang="ts">
	import TerminalController from '../terminal/TerminalController.svelte';

	interface Props {
		workspaceId: string;
		workspaceName: string;
		terminalId: string;
		active?: boolean;
		compact?: boolean;
	}

	const {
		workspaceId,
		workspaceName,
		terminalId,
		active = true,
		compact = false,
	}: Props = $props();

	let terminalContainer: HTMLDivElement | null = $state(null);
	let controller: {
		restart?: () => Promise<void>;
		retryHealthCheck?: () => void;
		focus?: () => void;
		scrollToBottom?: () => void;
		checkAtBottom?: () => boolean;
	} | null = $state(null);

	let hoveringBottomRight = $state(false);
	let notAtBottom = $state(false);
	let scrollCheckInterval: ReturnType<typeof setInterval> | null = null;

	$effect(() => {
		if (hoveringBottomRight) {
			notAtBottom = !(controller?.checkAtBottom?.() ?? true);
			scrollCheckInterval = setInterval(() => {
				notAtBottom = !(controller?.checkAtBottom?.() ?? true);
			}, 300);
		} else {
			if (scrollCheckInterval) {
				clearInterval(scrollCheckInterval);
				scrollCheckInterval = null;
			}
		}
		return () => {
			if (scrollCheckInterval) {
				clearInterval(scrollCheckInterval);
				scrollCheckInterval = null;
			}
		};
	});

	const handleScrollToBottom = (): void => {
		controller?.scrollToBottom?.();
		notAtBottom = false;
	};
	let controllerState = $state({
		status: '',
		message: '',
		health: 'unknown' as 'unknown' | 'checking' | 'ok' | 'stale',
		healthMessage: '',
		renderer: 'unknown' as 'unknown' | 'webgl',
		rendererMode: 'webgl' as const,
		sessiondAvailable: null as boolean | null,
		sessiondChecked: false,
		debugEnabled: false,
		debugStats: {
			bytesIn: 0,
			bytesOut: 0,
			backlog: 0,
			lastOutputAt: 0,
			lastCprAt: 0,
		},
	});

	const handleStateChange = (state: typeof controllerState): void => {
		controllerState = state;
	};

	const restartTerminal = async (): Promise<void> => {
		await controller?.restart?.();
	};

	const requestHealthCheck = (): void => {
		controller?.retryHealthCheck?.();
	};

	$effect(() => {
		if (!active) return;
		controller?.focus?.();
	});

	const activeStatus = $derived(controllerState.status);
	const activeMessage = $derived(controllerState.message);
	const activeHealth = $derived(controllerState.health);
	const activeHealthMessage = $derived(controllerState.healthMessage);
	const activeRenderer = $derived(controllerState.renderer);
	const sessiondAvailable = $derived(controllerState.sessiondAvailable);
	const debugEnabled = $derived(controllerState.debugEnabled);
	const debugStats = $derived(controllerState.debugStats);
</script>

<section class="terminal" class:compact>
	<TerminalController
		bind:this={controller}
		{workspaceId}
		{workspaceName}
		{terminalId}
		{active}
		{terminalContainer}
		onStateChange={handleStateChange}
	/>
	{#if !compact}
		<header class="terminal-header">
			<div class="title">Terminal</div>
			<div class="terminal-actions">
				<span
					class="health-indicator"
					class:ok={activeHealth === 'ok'}
					class:stale={activeHealth === 'stale'}
					class:checking={activeHealth === 'checking'}
					title="{sessiondAvailable === true
						? 'daemon'
						: sessiondAvailable === false
							? 'local'
							: 'checking'} | {activeRenderer} | {activeHealth}"
				></span>
				{#if debugEnabled}
					<div
						class="daemon-status"
						class:offline={sessiondAvailable === false}
						class:online={sessiondAvailable === true}
						title={sessiondAvailable === true
							? 'Session daemon active'
							: sessiondAvailable === false
								? 'Session daemon unavailable (using local shell)'
								: 'Checking session daemon status'}
					>
						{#if sessiondAvailable === true}
							daemon
						{:else if sessiondAvailable === false}
							local
						{:else}
							checking
						{/if}
					</div>
					<div class="renderer-status" title="Terminal renderer">
						{#if activeRenderer === 'webgl'}
							WebGL
						{:else}
							?
						{/if}
					</div>
				{/if}
			</div>
		</header>
	{/if}
	<div class="terminal-body">
		{#if activeStatus && activeStatus !== 'ready' && activeStatus !== 'standby'}
			<div class="terminal-status">
				<div class="status-text">
					{#if activeStatus === 'idle'}
						Terminal suspended due to inactivity.
					{:else if activeStatus === 'error'}
						Terminal error.
					{:else if activeStatus === 'closed'}
						Terminal closed.
					{:else if activeStatus === 'starting'}
						Starting terminal…
					{:else if activeStatus === 'standby'}
						Terminal is ready to start.
					{/if}
					{#if activeMessage}
						<span class="status-message">{activeMessage}</span>
					{/if}
				</div>
				<button class="restart" onclick={restartTerminal} type="button">Restart</button>
			</div>
		{/if}
		{#if activeHealthMessage && activeHealth !== 'ok'}
			<div class="terminal-status subtle">
				<div class="status-text">
					{activeHealthMessage}
				</div>
				<button class="restart" type="button" onclick={requestHealthCheck}> Retry check </button>
			</div>
		{/if}
		{#if debugEnabled}
			<div class="terminal-debug">
				<div>bytes in: {debugStats.bytesIn}</div>
				<div>bytes out: {debugStats.bytesOut}</div>
				<div>backlog: {debugStats.backlog}</div>
				<div>
					last output: {debugStats.lastOutputAt
						? new Date(debugStats.lastOutputAt).toLocaleTimeString()
						: '—'}
				</div>
				<div>
					last cpr: {debugStats.lastCprAt
						? new Date(debugStats.lastCprAt).toLocaleTimeString()
						: '—'}
				</div>
			</div>
		{/if}
		<div class="terminal-surface">
			<div class="terminal-mount" bind:this={terminalContainer}></div>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="scroll-hover-zone"
				onmouseenter={() => (hoveringBottomRight = true)}
				onmouseleave={() => (hoveringBottomRight = false)}
			>
				{#if hoveringBottomRight && notAtBottom}
					<button
						class="scroll-to-bottom"
						type="button"
						onclick={handleScrollToBottom}
						aria-label="Scroll to bottom"
					>
						<svg
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2.5"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<polyline points="6 9 12 15 18 9"></polyline>
						</svg>
					</button>
				{/if}
			</div>
		</div>
	</div>
</section>

<style>
	.terminal {
		display: flex;
		flex-direction: column;
		gap: 8px;
		height: 100%;
	}

	.terminal.compact {
		gap: 6px;
	}

	.terminal-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		min-height: 32px;
		gap: 8px;
	}

	.title {
		font-size: var(--text-md);
		font-weight: 500;
		color: var(--muted);
	}

	.terminal-body {
		background: var(--panel);
		border: none;
		border-radius: 0;
		padding: 0;
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 0;
		min-height: 0;
	}

	.terminal-actions {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.terminal.compact .terminal-body {
		padding: 0;
		border-radius: 0;
	}

	.daemon-status {
		font-size: var(--text-xs);
		color: var(--muted);
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 2px 8px;
		background: rgba(255, 255, 255, 0.02);
		letter-spacing: 0.02em;
	}

	.daemon-status.online {
		color: var(--success);
		border-color: color-mix(in srgb, var(--success) 50%, var(--border));
		background: color-mix(in srgb, var(--success) 12%, transparent);
	}

	.daemon-status.offline {
		color: var(--warning);
		border-color: color-mix(in srgb, var(--warning) 50%, var(--border));
		background: color-mix(in srgb, var(--warning) 12%, transparent);
	}

	.renderer-status {
		font-size: var(--text-xs);
		color: var(--muted);
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 2px 8px;
		background: rgba(255, 255, 255, 0.02);
		letter-spacing: 0.02em;
	}

	@keyframes pulse {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.4;
		}
	}

	.health-indicator {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--muted);
		cursor: help;
		transition: background var(--transition-fast);
	}

	.health-indicator.ok {
		background: var(--success);
	}

	.health-indicator.stale {
		background: var(--warning);
	}

	.health-indicator.checking {
		background: var(--accent);
		animation: pulse 1.5s ease-in-out infinite;
	}

	.terminal-status {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 12px;
		padding: 10px 12px;
		border-radius: var(--radius-md);
		border: 1px solid var(--warning-soft);
		background: var(--warning-subtle);
		color: var(--text);
		font-size: var(--text-sm);
	}

	.terminal-status.subtle {
		background: var(--panel-soft);
		border-color: var(--border);
	}

	.status-message {
		margin-left: 8px;
		color: var(--muted);
	}

	.restart {
		background: var(--accent);
		border: none;
		color: #081018;
		padding: 6px 10px;
		border-radius: var(--radius-sm);
		font-weight: 600;
		cursor: pointer;
		transition:
			background var(--transition-fast),
			transform var(--transition-fast);
	}

	.restart:hover:not(:disabled) {
		background: color-mix(in srgb, var(--accent) 85%, white);
	}

	.restart:active:not(:disabled) {
		transform: scale(0.98);
	}

	.terminal-debug {
		font-size: var(--text-xs);
		color: var(--muted);
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
		gap: 6px 12px;
		border: 1px dashed var(--border);
		border-radius: var(--radius-sm);
		padding: 8px;
		background: rgba(255, 255, 255, 0.02);
	}

	.terminal-surface {
		flex: 1;
		background: var(--panel-strong);
		border-radius: 10px;
		overflow: hidden;
		position: relative;
	}

	.terminal-mount {
		position: absolute;
		inset: 8px;
		z-index: 1;
	}

	.scroll-hover-zone {
		position: absolute;
		bottom: 0;
		right: 0;
		width: 64px;
		height: 64px;
		z-index: 10;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.scroll-to-bottom {
		width: 34px;
		height: 34px;
		border-radius: 50%;
		border: 1px solid var(--border);
		background: var(--panel-strong);
		color: var(--muted);
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		opacity: 0.8;
		transition:
			opacity var(--transition-fast),
			background var(--transition-fast),
			color var(--transition-fast);
	}

	.scroll-to-bottom:hover {
		opacity: 1;
		background: var(--accent);
		color: #081018;
		border-color: var(--accent);
	}

	:global(.terminal-instance) {
		position: absolute;
		inset: 0;
		opacity: 1;
		visibility: visible;
		pointer-events: auto;
		z-index: 1;
		overflow: hidden;
	}

	:global(.terminal-instance .kitty-layer) {
		position: absolute;
		inset: 0;
		pointer-events: none;
	}

	:global(.terminal-instance .kitty-underlay) {
		z-index: 0;
	}

	:global(.terminal-instance .xterm) {
		position: relative;
		z-index: 1;
	}

	:global(.terminal-instance .xterm-viewport) {
		background: transparent;
		scrollbar-width: none;
	}

	:global(.terminal-instance .xterm-viewport::-webkit-scrollbar) {
		display: none;
	}

	:global(.terminal-instance .kitty-overlay) {
		z-index: 2;
	}

	:global(.terminal-instance[data-active='true']) {
		visibility: visible;
		pointer-events: auto;
	}
</style>
