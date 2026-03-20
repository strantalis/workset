<script lang="ts">
	import { AlertCircle, FileCode, Loader2 } from '@lucide/svelte';
	import type { DiffLineAnnotation, ReviewAnnotation } from './annotations';
	import type {
		FileDiffRenderOptions,
		FileDiffRenderer,
		FileDiffRendererModule,
	} from './diffRenderController';
	import { buildDiffRenderOptions } from './diffRenderOptions';

	type ParsedFileDiff = Parameters<FileDiffRenderer<ReviewAnnotation>['render']>[0]['fileDiff'];

	type DiffsModule = FileDiffRendererModule<ReviewAnnotation> & {
		parsePatchFiles: (patch: string) => { files?: ParsedFileDiff[] }[];
	};

	interface Props {
		patch: string | null | undefined;
		loading: boolean;
		error: string | null;
		binary: boolean;
		truncated: boolean;
		totalLines: number;
		lineAnnotations?: DiffLineAnnotation<ReviewAnnotation>[];
		renderAnnotation?: (
			annotation: DiffLineAnnotation<ReviewAnnotation>,
		) => HTMLElement | undefined;
		onRenderError?: (message: string) => void;
	}

	const {
		patch,
		loading,
		error,
		binary,
		truncated,
		totalLines,
		lineAnnotations = [],
		renderAnnotation,
		onRenderError,
	}: Props = $props();

	let diffsModule: DiffsModule | null = $state(null);
	let diffContainer: HTMLElement | null = $state(null);
	let diffInstance: FileDiffRenderer<ReviewAnnotation> | null = $state(null);
	let diffRenderContainer: HTMLElement | null = $state(null);
	let diffRenderEpoch = 0;

	const buildDiffOptions = (
		container: HTMLElement | null = diffContainer,
	): FileDiffRenderOptions<ReviewAnnotation> =>
		buildDiffRenderOptions(container?.clientWidth, renderAnnotation);

	const ensureDiffsModule = async (): Promise<DiffsModule> => {
		if (diffsModule) return diffsModule;
		diffsModule = (await import('@pierre/diffs')) as unknown as DiffsModule;
		return diffsModule;
	};

	$effect(() => {
		const currentPatch = patch;
		const container = diffContainer;
		const annotations = lineAnnotations;
		if (!currentPatch || !container) return;
		const currentEpoch = ++diffRenderEpoch;

		void ensureDiffsModule().then((mod) => {
			if (currentEpoch !== diffRenderEpoch) return;
			if (!container.isConnected) return;
			if (patch !== currentPatch || diffContainer !== container) {
				return;
			}

			const parsed = mod.parsePatchFiles(currentPatch);
			const fileDiff = parsed[0]?.files?.[0] ?? null;
			if (!fileDiff) return;

			if (diffRenderContainer !== container) {
				diffInstance?.cleanUp();
				diffInstance = null;
				diffRenderContainer = container;
			}

			if (!diffInstance) {
				diffInstance = new mod.FileDiff(buildDiffOptions(container));
			} else {
				diffInstance.setOptions(buildDiffOptions(container));
			}
			if (currentEpoch !== diffRenderEpoch) return;
			if (!container.isConnected) return;
			if (patch !== currentPatch || diffContainer !== container) {
				return;
			}
			try {
				diffInstance.render({
					fileDiff,
					fileContainer: container,
					forceRender: true,
					lineAnnotations: annotations,
				});
			} catch (err) {
				// Guard against DOM races inside @pierre/diffs when container nodes were replaced.
				diffInstance?.cleanUp();
				diffInstance = new mod.FileDiff(buildDiffOptions(container));
				try {
					diffInstance.render({
						fileDiff,
						fileContainer: container,
						forceRender: true,
						lineAnnotations: annotations,
					});
				} catch (innerErr) {
					const renderErr = innerErr instanceof Error ? innerErr : err;
					const msg = renderErr instanceof Error ? renderErr.message : 'Failed to render diff.';
					onRenderError?.(msg);
				}
			}
		});
	});

	$effect(() => {
		return () => {
			diffInstance?.cleanUp();
			diffInstance = null;
			diffRenderContainer = null;
		};
	});
</script>

{#if error}
	<div class="diff-placeholder">
		<AlertCircle size={20} />
		<p>{error}</p>
	</div>
{:else if binary}
	<div class="diff-placeholder">
		<FileCode size={24} />
		<p>Binary file</p>
	</div>
{:else if patch}
	<div class="diff-renderer-wrap">
		<div class="diff-renderer">
			<diffs-container bind:this={diffContainer}></diffs-container>
		</div>
		{#if loading}
			<div class="diff-loading-overlay">
				<Loader2 size={18} class="spin" />
				<p>Refreshing diff...</p>
			</div>
		{/if}
	</div>
	{#if truncated}
		<div class="diff-truncated">
			Diff truncated ({totalLines} total lines)
		</div>
	{/if}
{:else if loading}
	<div class="diff-placeholder">
		<Loader2 size={20} class="spin" />
		<p>Loading diff...</p>
	</div>
{:else}
	<div class="diff-placeholder">
		<FileCode size={24} />
		<p>No diff content</p>
	</div>
{/if}

<style src="./DiffRenderer.css"></style>
