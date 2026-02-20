<script lang="ts">
	import { scale } from 'svelte/transition';
	import { GitBranch, LayoutTemplate } from '@lucide/svelte';

	export interface RepoNode {
		name: string;
		highlighted?: boolean;
	}

	interface Props {
		repos: RepoNode[];
		centerLabel: string;
		centerDim?: boolean;
	}

	const { repos, centerLabel, centerDim = false }: Props = $props();

	const repoPositions = $derived.by(() => {
		if (repos.length === 0) return [];
		const total = repos.length;
		const radius = total === 1 ? 120 : 140;
		return repos.map((repo, i) => {
			const angle = total === 1 ? 0 : (i / total) * 2 * Math.PI - Math.PI / 2;
			return {
				x: Math.cos(angle) * radius,
				y: Math.sin(angle) * radius,
				name: repo.name,
				highlighted: repo.highlighted ?? false,
			};
		});
	});
</script>

<div class="topology-container">
	<div class="topo-gradient"></div>

	<h3 class="topo-title ws-section-title">Workset Topology</h3>

	<div class="topo-area">
		<div class="topo-center-wrapper">
			<!-- SVG layer for animated connection lines -->
			<svg class="topo-svg" viewBox="-160 -160 320 320">
				{#each repoPositions as pos, i (pos.name + '-line')}
					<line
						x1="0"
						y1="0"
						x2={pos.x}
						y2={pos.y}
						class="topo-svg-line"
						class:highlighted={pos.highlighted}
						class:dimmed={!pos.highlighted}
						style="animation-delay: {i * 150}ms"
					/>
				{/each}
			</svg>

			<!-- Central hub -->
			<div class="hub-node" class:dim={centerDim}>
				<LayoutTemplate size={24} />
				<span class="hub-label">{centerLabel || '...'}</span>
			</div>

			<!-- Satellite repo nodes -->
			{#each repoPositions as pos, i (pos.name)}
				<div
					class="repo-node"
					class:highlighted={pos.highlighted}
					class:dimmed={!pos.highlighted}
					style="transform: translate({pos.x}px, {pos.y}px)"
					in:scale={{ duration: 250, delay: i * 100 }}
				>
					<GitBranch size={16} />
					<span class="repo-node-label">
						{pos.name.length > 8 ? pos.name.slice(0, 7) + '…' : pos.name}
					</span>
				</div>
			{/each}
		</div>
	</div>

	<div class="topo-footer">
		<div class="topo-badge">
			<GitBranch size={12} />
			<span>{repos.length} repo{repos.length !== 1 ? 's' : ''}</span>
		</div>
	</div>
</div>

<style>
	.topology-container {
		flex: 1;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 20px;
		display: flex;
		flex-direction: column;
		position: relative;
		overflow: hidden;
		min-height: 360px;
	}

	.topo-gradient {
		position: absolute;
		inset: 0;
		background: radial-gradient(
			circle at center,
			color-mix(in srgb, var(--accent) 5%, transparent) 0%,
			transparent 70%
		);
		pointer-events: none;
		animation: gradientPulse 4s ease-in-out infinite;
	}

	@keyframes gradientPulse {
		0%,
		100% {
			opacity: 0.6;
		}
		50% {
			opacity: 1;
		}
	}

	.topo-title {
		margin: 0;
		font-size: var(--text-sm);
		font-weight: 500;
		margin-bottom: 16px;
		position: relative;
		z-index: 1;
	}

	.topo-area {
		flex: 1;
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1;
	}

	.topo-center-wrapper {
		position: relative;
		width: 320px;
		height: 320px;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	/* ── Hub node ── */
	.hub-node {
		position: absolute;
		width: 96px;
		height: 96px;
		border-radius: 50%;
		background: var(--panel-soft);
		border: 2px solid var(--accent);
		box-shadow: 0 0 30px color-mix(in srgb, var(--accent) 30%, transparent);
		animation: hubPulse 3s ease-in-out infinite;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 4px;
		color: var(--accent);
		z-index: 20;
		transition:
			transform 0.3s,
			opacity 0.3s;
	}

	.hub-node.dim {
		transform: scale(0.8);
		opacity: 0.5;
	}

	.hub-label {
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
		color: var(--muted);
		text-align: center;
		max-width: 80px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	@keyframes hubPulse {
		0%,
		100% {
			box-shadow: 0 0 30px color-mix(in srgb, var(--accent) 25%, transparent);
		}
		50% {
			box-shadow:
				0 0 40px color-mix(in srgb, var(--accent) 40%, transparent),
				0 0 80px color-mix(in srgb, var(--accent) 10%, transparent);
		}
	}

	/* ── SVG connection lines ── */
	.topo-svg {
		position: absolute;
		width: 320px;
		height: 320px;
		pointer-events: none;
		overflow: visible;
	}

	.topo-svg-line {
		stroke: var(--accent);
		stroke-width: 2;
		stroke-dasharray: 6 4;
		stroke-linecap: round;
		opacity: 0.6;
		animation: dashFlow 1.2s linear infinite;
	}

	.topo-svg-line.dimmed {
		opacity: 0.25;
	}

	@keyframes dashFlow {
		to {
			stroke-dashoffset: -20;
		}
	}

	/* ── Repo nodes ── */
	.repo-node {
		position: absolute;
		left: 50%;
		top: 50%;
		width: 56px;
		height: 56px;
		margin-left: -28px;
		margin-top: -28px;
		border-radius: 12px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 2px;
		color: var(--muted);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
		transition:
			transform 0.25s ease,
			box-shadow 0.25s ease,
			border-color 0.25s ease,
			opacity 0.25s ease;
	}

	.repo-node.highlighted {
		border-color: var(--success);
		color: var(--success);
		box-shadow:
			0 4px 12px rgba(0, 0, 0, 0.3),
			0 0 16px color-mix(in srgb, var(--success) 25%, transparent);
	}

	.repo-node.dimmed {
		opacity: 0.5;
	}

	.repo-node.highlighted:hover {
		transform: translate(var(--tx, 0), var(--ty, 0)) scale(1.08);
		box-shadow:
			0 6px 20px rgba(0, 0, 0, 0.4),
			0 0 20px color-mix(in srgb, var(--success) 30%, transparent);
		border-color: var(--success);
	}

	.repo-node-label {
		font-size: var(--text-mono-xs);
		color: inherit;
		max-width: 50px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* ── Topo footer ── */
	.topo-footer {
		margin-top: 16px;
		text-align: center;
		position: relative;
		z-index: 1;
	}

	.topo-badge {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 6px 12px;
		border-radius: 999px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		color: var(--muted);
		font-size: var(--text-mono-sm);
		font-family: var(--font-mono);
	}

	/* ── Responsive ── */
	@media (max-width: 900px) {
		.topology-container {
			min-height: 300px;
		}
	}
</style>
