<script lang="ts">
	import { createTerminalPerformanceSampler } from '../terminal/terminalPerformance';
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
		focus?: () => void;
		scrollToBottom?: () => void;
		checkAtBottom?: () => boolean;
	} | null = $state(null);

	let hoveringBottomRight = $state(false);
	let notAtBottom = $state(false);
	let pendingScrollStateFrame: number | null = null;
	let performanceSnapshot = $state({
		fps: 0,
		frameTimeMs: 0,
		renderer: 'unknown',
	});

	const refreshScrollState = (): void => {
		notAtBottom = !(controller?.checkAtBottom?.() ?? true);
	};

	const scheduleScrollStateRefresh = (): void => {
		if (!hoveringBottomRight) return;
		if (pendingScrollStateFrame !== null) {
			cancelAnimationFrame(pendingScrollStateFrame);
		}
		pendingScrollStateFrame = requestAnimationFrame(() => {
			pendingScrollStateFrame = null;
			refreshScrollState();
		});
	};

	$effect(() => {
		return () => {
			if (pendingScrollStateFrame !== null) {
				cancelAnimationFrame(pendingScrollStateFrame);
				pendingScrollStateFrame = null;
			}
		};
	});

	const handleSurfaceMouseMove = (e: MouseEvent): void => {
		const target = e.currentTarget as HTMLElement;
		const rect = target.getBoundingClientRect();
		const inZone = rect.right - e.clientX <= 64 && rect.bottom - e.clientY <= 64;
		if (inZone && !hoveringBottomRight) {
			hoveringBottomRight = true;
			refreshScrollState();
		} else if (!inZone && hoveringBottomRight) {
			hoveringBottomRight = false;
			notAtBottom = false;
		}
	};

	const handleSurfaceMouseLeave = (): void => {
		hoveringBottomRight = false;
		notAtBottom = false;
		if (pendingScrollStateFrame !== null) {
			cancelAnimationFrame(pendingScrollStateFrame);
			pendingScrollStateFrame = null;
		}
	};

	const handleScrollToBottom = (): void => {
		controller?.scrollToBottom?.();
		notAtBottom = false;
	};

	const resolveRendererLabel = (): string => {
		const canvas = terminalContainer?.querySelector('canvas');
		return canvas?.dataset.terminalRenderer ?? 'unknown';
	};

	let controllerState = $state({
		status: '',
		message: '',
		health: 'unknown' as 'unknown' | 'checking' | 'ok' | 'stale',
		healthMessage: '',
		sessiondAvailable: null as boolean | null,
		sessiondChecked: false,
		debugEnabled: false,
		debugStats: {
			bytesIn: 0,
			bytesOut: 0,
			lastOutputAt: 0,
			lastCprAt: 0,
		},
	});

	const handleStateChange = (state: typeof controllerState): void => {
		controllerState = state;
	};

	$effect(() => {
		if (!active) return;
		controller?.focus?.();
	});

	const activeStatus = $derived(controllerState.status);
	const activeMessage = $derived(controllerState.message);
	const activeHealth = $derived(controllerState.health);
	const activeHealthMessage = $derived(controllerState.healthMessage);
	const sessiondAvailable = $derived(controllerState.sessiondAvailable);
	const debugEnabled = $derived(controllerState.debugEnabled);
	const debugStats = $derived(controllerState.debugStats);
	const rendererLabel = $derived(performanceSnapshot.renderer);
	const fpsLabel = $derived(Math.round(performanceSnapshot.fps));
	const frameTimeLabel = $derived(Math.round(performanceSnapshot.frameTimeMs * 10) / 10);

	$effect(() => {
		if (!active || !debugEnabled) {
			performanceSnapshot = {
				fps: 0,
				frameTimeMs: 0,
				renderer: resolveRendererLabel(),
			};
			return;
		}

		const sampler = createTerminalPerformanceSampler();
		let frameHandle = 0;
		let lastPublishedAt = 0;

		const updatePerformance = (timestamp: number): void => {
			const next = sampler.sampleFrame(timestamp);
			const renderer = resolveRendererLabel();
			if (
				lastPublishedAt === 0 ||
				timestamp - lastPublishedAt >= 250 ||
				performanceSnapshot.renderer !== renderer
			) {
				performanceSnapshot = {
					fps: next.fps,
					frameTimeMs: next.frameTimeMs,
					renderer,
				};
				lastPublishedAt = timestamp;
			}
			frameHandle = requestAnimationFrame(updatePerformance);
		};

		frameHandle = requestAnimationFrame(updatePerformance);
		return () => {
			cancelAnimationFrame(frameHandle);
			sampler.reset();
		};
	});
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
			<div class="title-group">
				<div class="title">Terminal</div>
			</div>
			<div class="terminal-actions">
				{#if activeHealth === 'stale' || activeHealth === 'checking'}
					<div
						class="health-badge"
						class:stale={activeHealth === 'stale'}
						class:checking={activeHealth === 'checking'}
						title="{sessiondAvailable === true
							? 'daemon'
							: sessiondAvailable === false
								? 'local'
								: 'checking'} | ghostty | {activeHealth}"
					>
						{#if activeHealth === 'stale'}
							stale
						{:else}
							checking
						{/if}
					</div>
				{/if}
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
				{/if}
			</div>
		</header>
	{/if}
	<div class="terminal-body">
		{#if activeStatus && activeStatus !== 'ready' && activeStatus !== 'standby'}
			<div
				class="terminal-status"
				class:is-error={activeStatus === 'error'}
				class:is-idle={activeStatus === 'idle'}
				class:is-closed={activeStatus === 'closed'}
				class:is-starting={activeStatus === 'starting'}
			>
				{#if activeStatus === 'starting'}
					<div class="status-content">
						<div class="status-spinner"></div>
						<span class="status-label">Starting terminal…</span>
					</div>
				{:else if activeStatus === 'error'}
					<div class="status-content">
						<span class="status-label">Terminal encountered an error</span>
						{#if activeMessage}
							<span class="status-detail">{activeMessage}</span>
						{/if}
					</div>
				{:else if activeStatus === 'idle'}
					<div class="status-content">
						<span class="status-label">Terminal paused due to inactivity</span>
						<span class="status-detail">Click or type to resume</span>
					</div>
				{:else if activeStatus === 'closed'}
					<div class="status-content">
						<span class="status-label">Terminal session ended</span>
						{#if activeMessage}
							<span class="status-detail">{activeMessage}</span>
						{/if}
					</div>
				{/if}
			</div>
		{/if}
		{#if activeHealthMessage && activeHealth !== 'ok'}
			<div class="terminal-status subtle">
				<div class="status-content">
					<span class="status-detail">{activeHealthMessage}</span>
				</div>
			</div>
		{/if}
		{#if debugEnabled}
			<div class="terminal-debug">
				<div>renderer: {rendererLabel}</div>
				<div>fps: {fpsLabel}</div>
				<div>frame: {frameTimeLabel} ms</div>
				<div>bytes in: {debugStats.bytesIn}</div>
				<div>bytes out: {debugStats.bytesOut}</div>
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
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="terminal-surface"
			onwheel={scheduleScrollStateRefresh}
			onmousemove={handleSurfaceMouseMove}
			onmouseleave={handleSurfaceMouseLeave}
		>
			<div class="terminal-mount" bind:this={terminalContainer}></div>
			<div class="scroll-hover-zone">
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
		gap: 0;
		height: 100%;
	}

	.terminal.compact {
		gap: 0;
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

	.title-group {
		display: flex;
		align-items: center;
		gap: 8px;
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
		gap: 8px;
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

	@keyframes pulse {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.4;
		}
	}

	/* Health badge — only shown when degraded (stale/checking) */
	.health-badge {
		font-size: var(--text-xs);
		color: var(--muted);
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 2px 8px;
		background: rgba(255, 255, 255, 0.02);
		letter-spacing: 0.02em;
	}

	.health-badge.stale {
		color: var(--warning);
		border-color: color-mix(in srgb, var(--warning) 50%, var(--border));
		background: color-mix(in srgb, var(--warning) 12%, transparent);
	}

	.health-badge.checking {
		color: var(--accent);
		border-color: color-mix(in srgb, var(--accent) 30%, var(--border));
		animation: pulse 1.5s ease-in-out infinite;
	}

	/* Terminal status banners */
	.terminal-status {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 12px;
		border-radius: 0;
		border-bottom: 1px solid var(--border);
		background: var(--panel-soft);
		color: var(--text);
		font-size: var(--text-sm);
	}

	.terminal-status.is-error {
		border-bottom-color: var(--danger-soft);
		background: color-mix(in srgb, var(--danger) 6%, var(--panel-soft));
	}

	.terminal-status.is-idle {
		border-bottom-color: var(--warning-soft);
		background: color-mix(in srgb, var(--warning) 4%, var(--panel-soft));
	}

	.terminal-status.is-closed {
		border-bottom-color: var(--border);
	}

	.terminal-status.is-starting {
		border-bottom-color: color-mix(in srgb, var(--accent) 30%, var(--border));
	}

	.terminal-status.subtle {
		background: var(--panel-soft);
		border-bottom-color: var(--border);
	}

	.status-content {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.status-label {
		font-weight: 500;
	}

	.status-detail {
		color: var(--muted);
		font-size: var(--text-xs);
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.status-spinner {
		width: 14px;
		height: 14px;
		border: 2px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	.terminal-debug {
		font-size: var(--text-xs);
		color: var(--muted);
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
		gap: 6px 12px;
		border: 1px dashed var(--border);
		border-radius: 0;
		padding: 8px;
		background: rgba(255, 255, 255, 0.02);
	}

	/* Terminal surface — edge-to-edge, no rounded corners */
	.terminal-surface {
		flex: 1;
		background: var(--panel-strong);
		border-radius: 0;
		overflow: hidden;
		position: relative;
		isolation: isolate;
		contain: layout paint;
	}

	/* Reduced inset padding — 4px instead of 8px for a more immersive feel */
	.terminal-mount {
		position: absolute;
		inset: 4px 0 4px 4px;
		z-index: 1;
		overflow: hidden;
		background: var(--panel-strong);
		border-radius: 0;
		isolation: isolate;
		contain: layout paint;
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
		pointer-events: none;
	}

	.scroll-to-bottom {
		pointer-events: auto;
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
		color: var(--on-accent);
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
		background: var(--panel-strong);
		isolation: isolate;
		contain: layout paint;
	}

	:global(.terminal-instance canvas) {
		display: block;
		background: var(--panel-strong);
	}

	/* ghostty terminal host */

	:global(.terminal-instance[data-active='true']) {
		visibility: visible;
		pointer-events: auto;
	}
</style>
