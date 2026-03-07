<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import Icon from '@iconify/svelte';
	import {
		ChevronDown,
		ChevronRight,
		Copy,
		FileText,
		FolderTree,
		GitBranch,
		LoaderCircle,
		Minus,
		Plus,
		Search,
		X,
	} from '@lucide/svelte';
	import { readWorkspaceRepoFile, searchWorkspaceRepoFiles } from '../api/repo-files';
	import { getDocumentViewerFileIcon } from './document-viewer/fileIcons';
	import { calculateMermaidOverlayFit } from './document-viewer/mermaidOverlay';
	import {
		buildDocumentViewerTree,
		buildExpandedKeysForQuery,
		shouldReplaceExpandedNodeSet,
		type DocumentViewerTreeNode,
	} from './document-viewer/tree';
	import {
		renderCodeDocument,
		renderMarkdownDocument,
		type DocumentRenderResult,
	} from '../documentRender';
	import type {
		DocumentRenderMode,
		DocumentSession,
		RepoFileContent,
		RepoFileSearchResult,
	} from '../types';

	interface Props {
		session: DocumentSession | null;
		onClose: () => void;
	}

	const { session, onClose }: Props = $props();

	let loading = $state(false);
	let renderLoading = $state(false);
	let treeLoading = $state(false);
	let error = $state<string | null>(null);
	let treeError = $state<string | null>(null);
	let file = $state<RepoFileContent | null>(null);
	let repoFiles = $state<RepoFileSearchResult[]>([]);
	let currentPath = $state('');
	let currentRepoId = $state('');
	let renderMode = $state<DocumentRenderMode>('raw');
	let rendered = $state<DocumentRenderResult>({ html: '', containsMermaid: false });
	let requestToken = 0;
	let renderToken = 0;
	let treeToken = 0;
	let copyFeedback = $state(false);
	let copyTimer: number | null = null;
	let showFileTree = $state(true);
	let expandedNodes = $state<Set<string>>(new Set());
	let searchQuery = $state('');
	let mermaidOverlayOpen = $state(false);
	let mermaidOverlayMarkup = $state('');
	let mermaidZoom = $state(1);
	let mermaidOffsetX = $state(0);
	let mermaidOffsetY = $state(0);
	let mermaidFitScale = $state(1);
	let mermaidDragging = $state(false);
	let mermaidDragPointerId = $state<number | null>(null);
	let mermaidDragOriginX = $state(0);
	let mermaidDragOriginY = $state(0);
	let mermaidDragStartOffsetX = $state(0);
	let mermaidDragStartOffsetY = $state(0);
	let mermaidOverlayCanvasEl = $state<HTMLElement | null>(null);
	let mermaidOverlayStageEl = $state<HTMLElement | null>(null);
	let mermaidIntrinsicWidth = $state(0);
	let mermaidIntrinsicHeight = $state(0);

	const hasSession = $derived(session !== null);
	const canRenderMarkdown = $derived(file?.isMarkdown === true);
	const isRenderedMarkdown = $derived(file?.isMarkdown === true && renderMode === 'rendered');
	const mermaidDiagnostics = $derived.by(() => {
		if (!isRenderedMarkdown || !file) return null;
		const fenceDetected = /(^|\n)\s*(```|~~~)\s*mermaid\b/i.test(file.content);
		if (!fenceDetected && !rendered.containsMermaid) return null;
		return {
			fenceDetected,
			containsMermaid: rendered.containsMermaid,
			hasSvg: rendered.html.includes('<svg'),
			hasError: rendered.html.includes('ws-mermaid-error'),
		};
	});

	const filteredRepoFiles = $derived.by<RepoFileSearchResult[]>(() => {
		const query = searchQuery.trim().toLowerCase();
		if (query.length === 0) return repoFiles;
		return repoFiles.filter((result) => {
			const fileName = result.path.split('/').pop()?.toLowerCase() ?? '';
			return fileName.includes(query) || result.path.toLowerCase().includes(query);
		});
	});

	$effect(() => {
		const query = searchQuery.trim().toLowerCase();
		if (query.length === 0) return;
		const toExpand = buildExpandedKeysForQuery(filteredRepoFiles);
		const next = new Set(expandedNodes);
		for (const key of toExpand) {
			next.add(key);
		}
		if (shouldReplaceExpandedNodeSet(expandedNodes, next)) {
			expandedNodes = next;
		}
	});

	const treeNodes = $derived.by<DocumentViewerTreeNode[]>(() =>
		buildDocumentViewerTree(filteredRepoFiles, expandedNodes),
	);

	const formatBytes = (sizeBytes: number): string => {
		if (!Number.isFinite(sizeBytes) || sizeBytes <= 0) return '0 B';
		if (sizeBytes < 1024) return `${sizeBytes} B`;
		if (sizeBytes < 1024 * 1024) return `${(sizeBytes / 1024).toFixed(1)} KB`;
		return `${(sizeBytes / (1024 * 1024)).toFixed(1)} MB`;
	};

	const clearCopyTimer = (): void => {
		if (copyTimer === null) return;
		window.clearTimeout(copyTimer);
		copyTimer = null;
	};

	const copyPath = async (): Promise<void> => {
		if (!file) return;
		await navigator.clipboard.writeText(`${file.repoName}/${file.path}`);
		copyFeedback = true;
		clearCopyTimer();
		copyTimer = window.setTimeout(() => {
			copyFeedback = false;
			copyTimer = null;
		}, 1200);
	};

	const selectTreeFile = (path: string, repoId: string): void => {
		currentPath = path;
		currentRepoId = repoId;
		mermaidOverlayOpen = false;
	};

	const closeMermaidOverlay = (): void => {
		mermaidOverlayOpen = false;
		mermaidOverlayMarkup = '';
		mermaidZoom = 1;
		mermaidFitScale = 1;
		mermaidOffsetX = 0;
		mermaidOffsetY = 0;
		mermaidDragging = false;
		mermaidDragPointerId = null;
		mermaidIntrinsicWidth = 0;
		mermaidIntrinsicHeight = 0;
	};

	const openMermaidOverlay = (svgMarkup: string): void => {
		if (!svgMarkup) return;
		mermaidOverlayMarkup = svgMarkup;
		mermaidZoom = 1;
		mermaidFitScale = 1;
		mermaidOffsetX = 0;
		mermaidOffsetY = 0;
		mermaidDragging = false;
		mermaidDragPointerId = null;
		mermaidIntrinsicWidth = 0;
		mermaidIntrinsicHeight = 0;
		mermaidOverlayOpen = true;
	};

	const adjustMermaidZoom = (delta: number): void => {
		mermaidZoom = Math.min(2.5, Math.max(0.5, Math.round((mermaidZoom + delta) * 100) / 100));
	};

	const resetMermaidZoom = (): void => {
		mermaidZoom = 1;
		mermaidOffsetX = 0;
		mermaidOffsetY = 0;
	};

	const updateMermaidOverlayFit = (): void => {
		if (!mermaidOverlayCanvasEl || !mermaidOverlayStageEl) return;
		const svg = mermaidOverlayStageEl.querySelector('svg');
		if (!(svg instanceof SVGElement)) return;
		const fit = calculateMermaidOverlayFit(
			svg,
			mermaidOverlayCanvasEl.clientWidth - 32,
			mermaidOverlayCanvasEl.clientHeight - 32,
		);
		if (!fit) return;
		mermaidIntrinsicWidth = fit.intrinsicWidth;
		mermaidIntrinsicHeight = fit.intrinsicHeight;
		mermaidFitScale = fit.fitScale;
	};

	const handleMermaidPointerDown = (event: PointerEvent): void => {
		const target = event.target;
		if (!(target instanceof Element)) return;
		if (!target.closest('svg')) return;
		mermaidDragging = true;
		mermaidDragPointerId = event.pointerId;
		mermaidDragOriginX = event.clientX;
		mermaidDragOriginY = event.clientY;
		mermaidDragStartOffsetX = mermaidOffsetX;
		mermaidDragStartOffsetY = mermaidOffsetY;
		(event.currentTarget as HTMLElement | null)?.setPointerCapture?.(event.pointerId);
		event.preventDefault();
	};

	const handleMermaidPointerMove = (event: PointerEvent): void => {
		if (!mermaidDragging || mermaidDragPointerId !== event.pointerId) return;
		mermaidOffsetX = mermaidDragStartOffsetX + (event.clientX - mermaidDragOriginX);
		mermaidOffsetY = mermaidDragStartOffsetY + (event.clientY - mermaidDragOriginY);
	};

	const handleMermaidPointerUp = (event: PointerEvent): void => {
		if (mermaidDragPointerId !== event.pointerId) return;
		(event.currentTarget as HTMLElement | null)?.releasePointerCapture?.(event.pointerId);
		mermaidDragging = false;
		mermaidDragPointerId = null;
	};

	const toggleNode = (key: string): void => {
		const next = new Set(expandedNodes);
		if (next.has(key)) {
			next.delete(key);
		} else {
			next.add(key);
		}
		expandedNodes = next;
	};

	$effect(() => {
		if (!session) {
			file = null;
			error = null;
			treeError = null;
			loading = false;
			treeLoading = false;
			renderLoading = false;
			repoFiles = [];
			currentPath = '';
			rendered = { html: '', containsMermaid: false };
			return;
		}
		currentPath = session.path;
		currentRepoId = session.repoId;
		treeError = null;
		expandedNodes = new Set();
		searchQuery = '';
		const currentTreeToken = ++treeToken;
		treeLoading = true;
		void searchWorkspaceRepoFiles(session.workspaceId, '', 5000, session.repoId)
			.then((next) => {
				if (currentTreeToken !== treeToken) return;
				repoFiles = next;
				treeLoading = false;
			})
			.catch((loadError) => {
				if (currentTreeToken !== treeToken) return;
				treeError =
					loadError instanceof Error ? loadError.message : 'Unable to load repository files.';
				repoFiles = [];
				treeLoading = false;
			});
	});

	$effect(() => {
		if (!session || !currentPath || !currentRepoId) {
			file = null;
			error = null;
			loading = false;
			renderLoading = false;
			rendered = { html: '', containsMermaid: false };
			return;
		}
		const currentToken = ++requestToken;
		loading = true;
		error = null;
		file = null;
		rendered = { html: '', containsMermaid: false };
		void readWorkspaceRepoFile(session.workspaceId, currentRepoId, currentPath)
			.then((next) => {
				if (currentToken !== requestToken) return;
				const previousWasMarkdown = file?.isMarkdown === true;
				file = next;
				renderMode = next.isMarkdown ? (previousWasMarkdown ? renderMode : 'rendered') : 'raw';
				loading = false;
			})
			.catch((loadError) => {
				if (currentToken !== requestToken) return;
				error =
					loadError instanceof Error ? loadError.message : 'Unable to read the selected file.';
				file = null;
				loading = false;
			});
	});

	$effect(() => {
		const selectedPath = currentPath;
		if (selectedPath.length > 0 || session === null) {
			// Reset copy feedback when the viewed document changes or closes.
		}
		clearCopyTimer();
		copyFeedback = false;
	});

	$effect(() => {
		if (!file || file.isBinary) {
			rendered = { html: '', containsMermaid: false };
			renderLoading = false;
			return;
		}
		const currentToken = ++renderToken;
		renderLoading = true;
		void (async () => {
			const next =
				renderMode === 'rendered' && file.isMarkdown
					? await renderMarkdownDocument(file.content)
					: await renderCodeDocument(file.content, file.path);
			if (currentToken !== renderToken) return;
			rendered = next;
			renderLoading = false;
		})().catch((renderError) => {
			if (currentToken !== renderToken) return;
			error =
				renderError instanceof Error ? renderError.message : 'Unable to render the selected file.';
			renderLoading = false;
		});
	});

	$effect(() => {
		if (!mermaidOverlayOpen || !mermaidOverlayMarkup) return;
		void tick().then(() => {
			updateMermaidOverlayFit();
		});
	});

	const handleKeydown = (event: KeyboardEvent): void => {
		if (!hasSession) return;
		if (mermaidOverlayOpen && event.key === 'Escape') {
			event.preventDefault();
			closeMermaidOverlay();
			return;
		}
		if (event.key === 'Escape') {
			event.preventDefault();
			onClose();
			return;
		}
		if ((event.metaKey || event.ctrlKey) && event.shiftKey && event.key.toLowerCase() === 'p') {
			event.preventDefault();
			showFileTree = !showFileTree;
		}
	};

	const handleDocumentClick = (event: MouseEvent): void => {
		const target = event.target;
		if (!(target instanceof Element)) return;
		const diagram = target.closest('.ws-mermaid-diagram');
		if (!(diagram instanceof HTMLElement)) return;
		const svg = diagram.querySelector('svg');
		if (!(svg instanceof SVGElement)) return;
		openMermaidOverlay(svg.outerHTML);
	};

	onDestroy(() => {
		clearCopyTimer();
	});
</script>

<svelte:window onkeydown={handleKeydown} />

{#if hasSession}
	<section class="viewer-panel" aria-label="Document viewer">
		<header class="viewer-header">
			<div class="viewer-meta">
				<span class="viewer-icon">
					{#if file?.isMarkdown}
						<FileText size={15} />
					{:else}
						<Icon icon="file-icons:default" width="15" />
					{/if}
				</span>
				<div class="viewer-titles">
					<div class="viewer-path">{currentPath}</div>
					<div class="viewer-subtitle">
						<span class="repo-pill">{session?.repoName}</span>
						{#if file}
							<span>{formatBytes(file.sizeBytes)}</span>
							{#if file.isTruncated}
								<span class="warning-pill">Truncated</span>
							{/if}
						{/if}
					</div>
				</div>
			</div>
			<div class="viewer-actions">
				<button
					type="button"
					class="icon-btn"
					class:active={showFileTree}
					aria-label={showFileTree ? 'Hide file tree' : 'Show file tree'}
					title={showFileTree ? 'Hide file tree (⌘⇧P)' : 'Show file tree (⌘⇧P)'}
					onclick={() => (showFileTree = !showFileTree)}
				>
					<FolderTree size={15} />
				</button>
				{#if canRenderMarkdown}
					<div class="mode-toggle" role="tablist" aria-label="Document render mode">
						<button
							type="button"
							class:active={renderMode === 'rendered'}
							role="tab"
							aria-selected={renderMode === 'rendered'}
							onclick={() => (renderMode = 'rendered')}
						>
							Rendered
						</button>
						<button
							type="button"
							class:active={renderMode === 'raw'}
							role="tab"
							aria-selected={renderMode === 'raw'}
							onclick={() => (renderMode = 'raw')}
						>
							Raw
						</button>
					</div>
				{/if}
				<button type="button" class="ghost-btn" onclick={() => void copyPath()}>
					<Copy size={13} />
					<span>{copyFeedback ? 'Copied' : 'Copy Path'}</span>
				</button>
				<button type="button" class="icon-btn" aria-label="Close document viewer" onclick={onClose}>
					<X size={15} />
				</button>
			</div>
		</header>

		<section class="viewer-shell" class:tree-visible={showFileTree}>
			{#if showFileTree}
				<aside class="viewer-tree" aria-label="Repository file tree">
					<div class="tree-header">
						<FolderTree size={13} />
						<span>Files</span>
					</div>
					<div class="tree-search">
						<Search size={14} />
						<input
							type="text"
							placeholder="Filter files..."
							bind:value={searchQuery}
							autocomplete="off"
							spellcheck="false"
						/>
					</div>
					{#if treeLoading}
						<div class="tree-state">
							<span class="spin"><LoaderCircle size={16} /></span>
							<span>Loading files…</span>
						</div>
					{:else if treeError}
						<div class="tree-state error">{treeError}</div>
					{:else if treeNodes.length === 0}
						<div class="tree-state">No files found.</div>
					{:else}
						<div class="tree-list">
							{#each treeNodes as node (node.key)}
								{#if node.kind === 'repo'}
									<button
										type="button"
										class="tree-repo"
										class:expanded={expandedNodes.has(node.key)}
										style={`--depth:${node.depth};`}
										onclick={() => toggleNode(node.key)}
									>
										{#if expandedNodes.has(node.key)}
											<ChevronDown size={11} />
										{:else}
											<ChevronRight size={11} />
										{/if}
										<GitBranch size={12} />
										<span>{node.label}</span>
									</button>
								{:else if node.kind === 'dir'}
									<button
										type="button"
										class="tree-dir"
										class:expanded={expandedNodes.has(node.key)}
										style={`--depth:${node.depth};`}
										onclick={() => toggleNode(node.key)}
									>
										{#if expandedNodes.has(node.key)}
											<ChevronDown size={11} />
										{:else}
											<ChevronRight size={11} />
										{/if}
										<span>{node.label}</span>
									</button>
								{:else}
									<button
										type="button"
										class="tree-file"
										class:selected={node.path === currentPath && node.repoId === currentRepoId}
										style={`--depth:${node.depth};`}
										onclick={() => selectTreeFile(node.path, node.repoId)}
									>
										<span class="file-icon" data-icon={getDocumentViewerFileIcon(node.path)}>
											<Icon icon={getDocumentViewerFileIcon(node.path)} width="12" />
										</span>
										<span>{node.label}</span>
									</button>
								{/if}
							{/each}
						</div>
					{/if}
				</aside>
			{/if}

			<section class="viewer-content" class:code-view={!isRenderedMarkdown}>
				{#if mermaidDiagnostics}
					<div class="mermaid-debug">
						<strong>Mermaid debug</strong>
						<span>fence: {mermaidDiagnostics.fenceDetected ? 'yes' : 'no'}</span>
						<span>detected: {mermaidDiagnostics.containsMermaid ? 'yes' : 'no'}</span>
						<span>svg: {mermaidDiagnostics.hasSvg ? 'yes' : 'no'}</span>
						<span>error: {mermaidDiagnostics.hasError ? 'yes' : 'no'}</span>
					</div>
				{/if}
				{#if loading}
					<div class="state-panel">
						<span class="spin"><LoaderCircle size={18} /></span>
						<span>Loading file…</span>
					</div>
				{:else if error}
					<div class="state-panel error">{error}</div>
				{:else if file?.isBinary}
					<div class="state-panel">
						This file looks binary. Workset will not render binary content inline.
					</div>
				{:else if renderLoading}
					<div class="state-panel">
						<span class="spin"><LoaderCircle size={18} /></span>
						<span>Rendering document…</span>
					</div>
				{:else if rendered.html}
					<div
						class="document-html"
						class:markdown={isRenderedMarkdown}
						class:code-view={!isRenderedMarkdown}
						role="presentation"
						onclick={handleDocumentClick}
						onkeydown={(event) => {
							if (event.key === 'Enter' || event.key === ' ') {
								handleDocumentClick(event as unknown as MouseEvent);
							}
						}}
					>
						<!-- eslint-disable-next-line svelte/no-at-html-tags -->
						{@html rendered.html}
					</div>
				{:else}
					<div class="state-panel">Nothing to render for this file.</div>
				{/if}
			</section>
		</section>
	</section>
{/if}

{#if mermaidOverlayOpen}
	<div
		class="mermaid-overlay"
		role="button"
		tabindex="0"
		aria-label="Close expanded Mermaid diagram"
		onclick={closeMermaidOverlay}
		onkeydown={(event) => {
			if (event.key === 'Escape') closeMermaidOverlay();
		}}
	>
		<div
			class="mermaid-overlay-panel"
			role="dialog"
			aria-modal="true"
			aria-label="Expanded Mermaid diagram"
			tabindex="-1"
			onclick={(event) => event.stopPropagation()}
			onkeydown={(event) => event.stopPropagation()}
		>
			<div class="mermaid-overlay-toolbar">
				<div class="mermaid-overlay-actions">
					<button
						type="button"
						class="icon-btn"
						aria-label="Zoom out"
						onclick={() => adjustMermaidZoom(-0.1)}
					>
						<Minus size={15} />
					</button>
					<button type="button" class="ghost-btn" onclick={resetMermaidZoom}>
						{Math.round(mermaidZoom * 100)}%
					</button>
					<button
						type="button"
						class="icon-btn"
						aria-label="Zoom in"
						onclick={() => adjustMermaidZoom(0.1)}
					>
						<Plus size={15} />
					</button>
				</div>
				<button
					type="button"
					class="icon-btn"
					aria-label="Close expanded diagram"
					onclick={closeMermaidOverlay}
				>
					<X size={15} />
				</button>
			</div>
			<div class="mermaid-overlay-canvas">
				<div
					bind:this={mermaidOverlayCanvasEl}
					class="mermaid-overlay-canvas-surface"
					class:dragging={mermaidDragging}
					role="presentation"
					onpointerdown={handleMermaidPointerDown}
					onpointermove={handleMermaidPointerMove}
					onpointerup={handleMermaidPointerUp}
					onpointercancel={handleMermaidPointerUp}
				>
					<div
						bind:this={mermaidOverlayStageEl}
						class="mermaid-overlay-stage"
						style={`--mermaid-scale:${mermaidFitScale * mermaidZoom}; --mermaid-x:${mermaidOffsetX}px; --mermaid-y:${mermaidOffsetY}px; --mermaid-width:${mermaidIntrinsicWidth}px; --mermaid-height:${mermaidIntrinsicHeight}px;`}
					>
						<!-- eslint-disable-next-line svelte/no-at-html-tags -->
						{@html mermaidOverlayMarkup}
					</div>
				</div>
			</div>
		</div>
	</div>
{/if}

<style src="./DocumentViewer.css"></style>
