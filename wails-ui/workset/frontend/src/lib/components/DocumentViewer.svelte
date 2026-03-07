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

	const getFileIcon = (path: string): string => {
		const ext = path.split('.').pop()?.toLowerCase() ?? '';
		const fileName = path.split('/').pop()?.toLowerCase() ?? '';

		if (['md', 'mdx', 'markdown'].includes(ext)) return 'file-icons:markdown';
		if (['json', 'jsonc', 'json5'].includes(ext)) return 'file-icons:json-1';
		if (['yaml', 'yml'].includes(ext)) return 'file-icons:yaml';
		if (['toml', 'ini', 'conf', 'config', 'env', 'editorconfig'].includes(ext)) return 'file-icons:config';
		if (['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'ico', 'bmp'].includes(ext)) return 'file-icons:image';
		if (['zip', 'tar', 'gz', 'rar', '7z', 'bz2', 'xz'].includes(ext)) return 'file-icons:archive';
		if (['csv', 'xlsx', 'xls', 'tsv'].includes(ext)) return 'file-icons:excel';
		if (['lock', 'sum', 'mod'].includes(ext)) return 'file-icons:lock';
		if (['dockerfile', 'makefile', 'rakefile', 'gemfile', 'procfile'].includes(fileName)) return 'file-icons:config';
		if (['license', 'readme', 'changelog', 'authors', 'contributors'].includes(fileName)) return 'file-icons:license';
		if (['gitignore', 'gitattributes', 'gitmodules'].includes(fileName)) return 'file-icons:git';
		if (['npmrc', 'npmignore', 'eslintrc', 'prettierrc'].includes(fileName)) return 'file-icons:npm';

		if (ext === 'go') return 'file-icons:go';
		if (['ts', 'tsx'].includes(ext)) return 'file-icons:typescript';
		if (['js', 'jsx', 'mjs', 'cjs'].includes(ext)) return 'file-icons:javascript';
		if (ext === 'py') return 'file-icons:python';
		if (ext === 'rs') return 'file-icons:rust';
		if (ext === 'java') return 'file-icons:java';
		if (ext === 'kt' || ext === 'kts') return 'file-icons:kotlin';
		if (ext === 'swift') return 'file-icons:swift';
		if (ext === 'rb' || ext === 'rake') return 'file-icons:ruby';
		if (ext === 'php') return 'file-icons:php';
		if (ext === 'c' || ext === 'h') return 'file-icons:c';
		if (['cpp', 'cc', 'cxx', 'hpp', 'hxx'].includes(ext)) return 'file-icons:cpp';
		if (ext === 'cs') return 'file-icons:csharp';
		if (ext === 'scala' || ext === 'sc') return 'file-icons:scala';
		if (ext === 'lua') return 'file-icons:lua';
		if (ext === 'r') return 'file-icons:r';
		if (ext === 'sh' || ext === 'bash' || ext === 'zsh') return 'file-icons:terminal';
		if (ext === 'ps1' || ext === 'psm1') return 'file-icons:powershell';
		if (ext === 'sql') return 'file-icons:database';
		if (ext === 'vue') return 'file-icons:vue';
		if (ext === 'svelte') return 'file-icons:svelte';
		if (ext === 'css' || ext === 'scss' || ext === 'sass' || ext === 'less') return 'file-icons:css';
		if (ext === 'html' || ext === 'htm') return 'file-icons:html';
		if (ext === 'xml' || ext === 'xhtml') return 'file-icons:xml';
		if (ext === 'graphql' || ext === 'gql') return 'file-icons:graphql';
		if (ext === 'proto') return 'file-icons:proto';
		if (ext === 'sol') return 'file-icons:solidity';
		if (ext === 'ex' || ext === 'exs') return 'file-icons:elixir';
		if (ext === 'erl') return 'file-icons:erlang';
		if (ext === 'hs') return 'file-icons:haskell';
		if (ext === 'ml' || ext === 'mli') return 'file-icons:ocaml';
		if (ext === 'clj' || ext === 'cljs') return 'file-icons:clojure';
		if (ext === 'lisp' || ext === 'lsp') return 'file-icons:lisp';
		if (ext === 'dart') return 'file-icons:dart';
		if (ext === 'elm') return 'file-icons:elm';
		if (ext === 'fsharp' || ext === 'fs') return 'file-icons:fsharp';
		if (ext === 'vim') return 'file-icons:vim';
		if (ext === 'perl' || ext === 'pl' || ext === 'pm') return 'file-icons:perl';
		if (ext === 'awk') return 'file-icons:awk';
		if (ext === 'wasm') return 'file-icons:wasm';
		if (ext === 'dockerfile' || fileName === 'dockerfile') return 'file-icons:docker';

		return 'file-icons:default';
	};

	type TreeNode =
		| { kind: 'repo'; key: string; label: string; repoId: string; depth: number }
		| { kind: 'dir'; key: string; label: string; depth: number }
		| { kind: 'file'; key: string; label: string; depth: number; path: string; repoId: string; isMarkdown: boolean };

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

		const toExpand = new Set<string>();
		for (const result of filteredRepoFiles) {
			const parts = result.path.split('/');
			if (parts.length > 1) {
				for (let i = 1; i < parts.length; i += 1) {
					const dirPath = parts.slice(0, i).join('/');
					toExpand.add(`dir:${result.repoId}:${dirPath}`);
				}
			}
			toExpand.add(`repo:${result.repoId}`);
		}

		const next = new Set(expandedNodes);
		for (const key of toExpand) {
			next.add(key);
		}
		if (
			next.size !== expandedNodes.size ||
			Array.from(next).some((key) => !expandedNodes.has(key))
		) {
			expandedNodes = next;
		}
	});

	const treeNodes = $derived.by<TreeNode[]>(() => {
		const nodes: TreeNode[] = [];
		const byRepo = new Map<string, RepoFileSearchResult[]>();
		
		for (const result of filteredRepoFiles) {
			const existing = byRepo.get(result.repoId);
			if (existing) {
				existing.push(result);
			} else {
				byRepo.set(result.repoId, [result]);
			}
		}

		const sortedRepos = [...byRepo.entries()].sort((a, b) => 
			(a[1][0]?.repoName ?? '').localeCompare(b[1][0]?.repoName ?? '')
		);

		const expanded = expandedNodes;

		for (const [repoId, files] of sortedRepos) {
			const repoName = files[0]?.repoName ?? repoId;
			const repoKey = `repo:${repoId}`;
			nodes.push({
				kind: 'repo',
				key: repoKey,
				label: repoName,
				repoId,
				depth: 0,
			});

			if (!expanded.has(repoKey)) continue;

			const seenDirs = new Set<string>();
			const sortedFiles = [...files].sort((a, b) => a.path.localeCompare(b.path));

			for (const result of sortedFiles) {
				const parts = result.path.split('/');
				if (parts.length > 1) {
					for (let i = 1; i < parts.length; i += 1) {
						const currentDir = parts.slice(0, i).join('/');
						const dirKey = `dir:${repoId}:${currentDir}`;
						if (seenDirs.has(dirKey)) continue;
						seenDirs.add(dirKey);

						const parentKey = i === 1 ? repoKey : `dir:${repoId}:${parts.slice(0, i - 1).join('/')}`;
						if (!expanded.has(parentKey)) continue;

						nodes.push({
							kind: 'dir',
							key: dirKey,
							label: parts[i - 1] ?? '',
							depth: i,
						});
					}
				}

				const parentKey = parts.length > 1 
					? `dir:${repoId}:${parts.slice(0, parts.length - 1).join('/')}`
					: repoKey;
				if (!expanded.has(parentKey)) continue;

				nodes.push({
					kind: 'file',
					key: `file:${repoId}:${result.path}`,
					label: parts[parts.length - 1] ?? result.path,
					depth: Math.max(1, parts.length),
					path: result.path,
					repoId: result.repoId,
					isMarkdown: result.isMarkdown,
				});
			}
		}
		return nodes;
	});

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
		const viewBox = svg.getAttribute('viewBox')?.trim() ?? '';
		let width = Number(svg.getAttribute('width') ?? '');
		let height = Number(svg.getAttribute('height') ?? '');
		if ((!Number.isFinite(width) || width <= 0 || !Number.isFinite(height) || height <= 0) && viewBox) {
			const parts = viewBox.split(/[\s,]+/).map(Number);
			if (parts.length === 4 && parts.every((value) => Number.isFinite(value))) {
				width = parts[2] ?? width;
				height = parts[3] ?? height;
			}
		}
		if (!Number.isFinite(width) || width <= 0 || !Number.isFinite(height) || height <= 0) return;
		mermaidIntrinsicWidth = width;
		mermaidIntrinsicHeight = height;
		const viewportWidth = Math.max(1, mermaidOverlayCanvasEl.clientWidth - 32);
		const viewportHeight = Math.max(1, mermaidOverlayCanvasEl.clientHeight - 32);
		mermaidFitScale = Math.min(viewportWidth / width, viewportHeight / height);
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
										<span class="file-icon" data-icon={getFileIcon(node.path)}>
											<Icon icon={getFileIcon(node.path)} width="12" />
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
					<button type="button" class="icon-btn" aria-label="Zoom out" onclick={() => adjustMermaidZoom(-0.1)}>
						<Minus size={15} />
					</button>
					<button type="button" class="ghost-btn" onclick={resetMermaidZoom}>
						{Math.round(mermaidZoom * 100)}%
					</button>
					<button type="button" class="icon-btn" aria-label="Zoom in" onclick={() => adjustMermaidZoom(0.1)}>
						<Plus size={15} />
					</button>
				</div>
				<button type="button" class="icon-btn" aria-label="Close expanded diagram" onclick={closeMermaidOverlay}>
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

<style>
	.viewer-panel {
		width: 100%;
		height: 100%;
		display: grid;
		grid-template-rows: auto minmax(0, 1fr);
		background: color-mix(in srgb, var(--panel) 94%, rgba(10, 14, 20, 0.96));
		border-left: 1px solid color-mix(in srgb, var(--border) 70%, transparent);
		box-shadow:
			-8px 0 24px rgba(0, 0, 0, 0.12),
			-2px 0 8px rgba(0, 0, 0, 0.08);
	}

	.viewer-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 14px;
		padding: 12px 14px;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 64%, transparent);
	}

	.viewer-meta {
		display: flex;
		align-items: flex-start;
		gap: 10px;
		min-width: 0;
	}

	.viewer-icon {
		color: color-mix(in srgb, var(--accent) 72%, white);
		display: inline-flex;
		align-items: center;
		padding-top: 2px;
	}

	.viewer-titles {
		min-width: 0;
		display: grid;
		gap: 5px;
	}

	.viewer-path {
		font-family: 'SF Mono', 'Monaco', monospace;
		font-size: 13px;
		color: var(--text);
		word-break: break-word;
	}

	.viewer-subtitle {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		font-size: 11px;
		color: var(--muted);
	}

	.repo-pill,
	.warning-pill {
		padding: 2px 8px;
		border-radius: 999px;
	}

	.repo-pill {
		background: color-mix(in srgb, var(--panel-strong) 84%, transparent);
		color: var(--text);
	}

	.warning-pill {
		background: color-mix(in srgb, var(--warning) 16%, transparent);
		color: color-mix(in srgb, var(--warning) 75%, white);
	}

	.viewer-actions {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.mode-toggle {
		display: inline-flex;
		padding: 4px;
		border-radius: 10px;
		background: color-mix(in srgb, var(--panel-strong) 86%, transparent);
	}

	.mode-toggle button,
	.ghost-btn,
	.icon-btn,
	.tree-repo,
	.tree-dir,
	.tree-file {
		border: none;
		cursor: pointer;
	}

	.mode-toggle button {
		padding: 6px 10px;
		border-radius: 8px;
		background: transparent;
		color: var(--muted);
		font-size: 11px;
		font-weight: 600;
	}

	.mode-toggle button.active {
		background: color-mix(in srgb, var(--accent) 18%, var(--panel-soft));
		color: var(--text);
	}

	.ghost-btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 7px 10px;
		border-radius: 10px;
		background: color-mix(in srgb, var(--panel-strong) 86%, transparent);
		color: var(--text);
		font-size: 12px;
	}

	.icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 10px;
		background: color-mix(in srgb, var(--panel-strong) 86%, transparent);
		color: var(--muted);
	}

	.ghost-btn:hover,
	.icon-btn:hover,
	.tree-repo:hover,
	.tree-dir:hover,
	.tree-file:hover {
		background: color-mix(in srgb, var(--accent) 12%, var(--panel-strong));
		color: var(--text);
	}

	.viewer-shell {
		min-height: 0;
		display: grid;
		grid-template-columns: minmax(0, 1fr);
	}

	.viewer-shell.tree-visible {
		grid-template-columns: minmax(220px, 240px) minmax(0, 1fr);
	}

	.viewer-tree {
		border-right: 1px solid color-mix(in srgb, var(--border) 64%, transparent);
		min-height: 0;
		display: flex;
		flex-direction: column;
		background: color-mix(in srgb, var(--panel-strong) 40%, transparent);
		overflow: hidden;
	}

	.tree-header {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 10px 12px;
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--muted);
		border-bottom: 1px solid color-mix(in srgb, var(--border) 64%, transparent);
	}

	.tree-search {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 64%, transparent);
		color: var(--muted);
	}

	.tree-search input {
		flex: 1;
		border: none;
		outline: none;
		background: transparent;
		color: var(--text);
		font-size: 12px;
		font-family: var(--font-mono);
	}

	.tree-search input::placeholder {
		color: var(--muted);
		opacity: 0.6;
	}

	.tree-list {
		padding: 6px;
		overflow: auto;
		min-height: 0;
		display: grid;
		gap: 2px;
	}

	.tree-repo,
	.tree-dir,
	.tree-file {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		text-align: left;
		padding: 5px 8px;
		padding-left: calc(10px + (var(--depth, 0) * 14px));
		border-radius: 8px;
		background: transparent;
		color: var(--muted);
		font-size: 12px;
		font-family: var(--font-mono);
	}

	.tree-repo {
		font-weight: 600;
		color: var(--text);
		margin-top: 4px;
	}

	.tree-repo:first-child {
		margin-top: 0;
	}

	.tree-dir {
		color: color-mix(in srgb, var(--muted) 85%, white);
	}

	.tree-file.selected {
		background: color-mix(in srgb, var(--accent) 12%, var(--panel-soft));
		color: var(--text);
	}

	.tree-state {
		padding: 14px 12px;
		display: flex;
		align-items: center;
		gap: 8px;
		color: var(--muted);
		font-size: 12px;
	}

	.tree-state.error {
		color: color-mix(in srgb, var(--status-error) 72%, white);
	}

	.viewer-content {
		min-height: 0;
		overflow: auto;
		padding: 14px;
	}

	.viewer-content.code-view {
		padding: 0;
		overflow: hidden;
	}

	.state-panel {
		min-height: 260px;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 10px;
		color: var(--muted);
		text-align: center;
		padding: 24px;
		font-size: 12px;
	}

	.state-panel.error {
		color: color-mix(in srgb, var(--status-error) 72%, white);
	}

	.mermaid-debug {
		display: inline-flex;
		flex-wrap: wrap;
		gap: 10px;
		margin: 0 0 12px;
		padding: 8px 10px;
		border-radius: 10px;
		background: color-mix(in srgb, var(--warning) 10%, transparent);
		border: 1px solid color-mix(in srgb, var(--warning) 22%, transparent);
		color: color-mix(in srgb, var(--warning) 78%, white);
		font-size: 11px;
		font-family: var(--font-mono);
	}

	.mermaid-debug strong {
		color: var(--text);
		font-weight: 700;
	}

	.document-html {
		color: var(--text);
		font-size: 13px;
	}

	.document-html.code-view {
		height: 100%;
		display: flex;
		min-height: 0;
		overflow: hidden;
	}

	.document-html.markdown {
		font-size: 13px;
		line-height: 1.68;
	}

	.document-html.markdown :global(> *:first-child) {
		margin-top: 0;
	}

	.document-html :global(h1),
	.document-html :global(h2),
	.document-html :global(h3),
	.document-html :global(h4) {
		color: var(--text);
	}

	.document-html.markdown :global(h1) {
		margin: 0 0 14px;
		font-size: 22px;
		line-height: 1.15;
		letter-spacing: -0.02em;
	}

	.document-html.markdown :global(h2) {
		margin: 24px 0 10px;
		padding-top: 4px;
		font-size: 17px;
		line-height: 1.2;
		border-top: 1px solid color-mix(in srgb, var(--border) 55%, transparent);
	}

	.document-html.markdown :global(h3) {
		margin: 18px 0 8px;
		font-size: 14px;
		line-height: 1.25;
	}

	.document-html.markdown :global(h4) {
		margin: 16px 0 6px;
		font-size: 13px;
		line-height: 1.25;
		color: color-mix(in srgb, var(--text) 82%, var(--muted));
	}

	.document-html :global(p),
	.document-html :global(li) {
		line-height: 1.58;
	}

	.document-html.markdown :global(p) {
		margin: 0 0 12px;
	}

	.document-html.markdown :global(ul),
	.document-html.markdown :global(ol) {
		margin: 0 0 14px;
		padding-left: 22px;
	}

	.document-html.markdown :global(li) {
		margin: 0 0 5px;
	}

	.document-html.markdown :global(strong) {
		font-weight: 700;
		color: color-mix(in srgb, var(--text) 88%, white);
	}

	.document-html.markdown :global(hr) {
		border: none;
		border-top: 1px solid color-mix(in srgb, var(--border) 55%, transparent);
		margin: 18px 0;
	}

	.document-html.markdown :global(blockquote) {
		margin: 0 0 14px;
		padding: 8px 12px;
		border-left: 3px solid color-mix(in srgb, var(--accent) 45%, transparent);
		background: color-mix(in srgb, var(--panel-soft) 82%, transparent);
		color: color-mix(in srgb, var(--muted) 88%, white);
		border-radius: 0 10px 10px 0;
	}

	.document-html.markdown :global(table) {
		width: 100%;
		border-collapse: collapse;
		margin: 0 0 16px;
		font-size: 12px;
	}

	.document-html.markdown :global(th),
	.document-html.markdown :global(td) {
		padding: 8px 10px;
		border: 1px solid color-mix(in srgb, var(--border) 55%, transparent);
		vertical-align: top;
	}

	.document-html.markdown :global(th) {
		background: color-mix(in srgb, var(--panel-strong) 78%, transparent);
		text-align: left;
	}

	.document-html :global(a) {
		color: color-mix(in srgb, var(--accent) 82%, white);
	}

	.document-html.markdown :global(code:not(pre code)) {
		padding: 1px 5px;
		border-radius: 6px;
		background: color-mix(in srgb, var(--panel-strong) 82%, transparent);
		border: 1px solid color-mix(in srgb, var(--border) 45%, transparent);
		font-size: 12px;
	}

	.document-html :global(pre.shiki) {
		margin: 0;
		border-radius: 12px;
		background: #111827;
		border: 1px solid color-mix(in srgb, var(--border) 55%, transparent);
		overflow-x: auto;
	}

	.document-html.code-view :global(pre.shiki) {
		flex: 1;
		height: 100%;
		border: none;
		border-radius: 0;
		background: transparent !important;
		overflow: auto;
	}

	.document-html :global(pre.shiki code) {
		counter-reset: line;
		display: block;
		min-width: max-content;
		padding: 14px 16px;
		font-size: 11px;
		line-height: 1.55;
		background: transparent;
	}

	.document-html.code-view :global(pre.shiki code) {
		min-height: 100%;
		padding: 12px 18px 24px;
	}

	.document-html :global(pre.shiki .line) {
		display: block;
		padding-left: 54px;
		position: relative;
	}

	.document-html :global(pre.shiki .line::before) {
		content: counter(line);
		counter-increment: line;
		position: absolute;
		left: 0;
		width: 36px;
		text-align: right;
		color: rgba(148, 163, 184, 0.72);
	}

	.document-html :global(.ws-mermaid-block) {
		display: grid;
		gap: 10px;
		padding: 14px;
		border-radius: 12px;
		background: color-mix(in srgb, var(--panel-soft) 82%, transparent);
		border: 1px solid color-mix(in srgb, var(--border) 55%, transparent);
		overflow: auto;
	}

	.document-html :global(.ws-mermaid-block .mermaid),
	.document-html :global(.ws-mermaid-diagram) {
		display: flex;
		justify-content: center;
		min-width: max-content;
		cursor: zoom-in;
	}

	.document-html :global(.ws-mermaid-block .mermaid svg),
	.document-html :global(.ws-mermaid-diagram svg) {
		max-width: 100%;
		height: auto;
	}

	.document-html :global(.ws-mermaid-error) {
		color: color-mix(in srgb, var(--status-error) 72%, white);
		font-size: 11px;
	}

	.mermaid-overlay {
		position: fixed;
		inset: 0;
		z-index: 650;
		background: rgba(5, 10, 18, 0.72);
		backdrop-filter: blur(8px);
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 28px;
	}

	.mermaid-overlay-panel {
		width: min(96vw, 1280px);
		height: min(92vh, 880px);
		display: grid;
		grid-template-rows: auto minmax(0, 1fr);
		background: color-mix(in srgb, var(--panel) 96%, rgba(5, 10, 18, 0.94));
		border: 1px solid color-mix(in srgb, var(--border) 70%, transparent);
		border-radius: 16px;
		overflow: hidden;
		box-shadow: 0 24px 80px rgba(0, 0, 0, 0.45);
	}

	.mermaid-overlay-toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 12px;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 64%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 78%, transparent);
	}

	.mermaid-overlay-actions {
		display: inline-flex;
		align-items: center;
		gap: 8px;
	}

	.mermaid-overlay-canvas {
		min-height: 0;
		padding: 20px;
		overflow: hidden;
	}

	.mermaid-overlay-canvas-surface {
		width: 100%;
		height: 100%;
		overflow: hidden;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: grab;
		user-select: none;
	}

	.mermaid-overlay-canvas-surface.dragging {
		cursor: grabbing;
	}

	.mermaid-overlay-stage {
		flex: 0 0 auto;
		display: flex;
		align-items: center;
		justify-content: center;
		transform: translate(var(--mermaid-x, 0px), var(--mermaid-y, 0px));
		will-change: transform;
	}

	.mermaid-overlay-stage :global(svg) {
		display: block;
		width: calc(var(--mermaid-width, 0px) * var(--mermaid-scale, 1));
		height: calc(var(--mermaid-height, 0px) * var(--mermaid-scale, 1));
		max-width: none;
		max-height: none;
		shape-rendering: geometricPrecision;
		text-rendering: geometricPrecision;
	}

	.spin {
		animation: spin 0.9s linear infinite;
	}

	@keyframes spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}

	@media (max-width: 1180px) {
		.viewer-shell {
			grid-template-columns: 1fr;
			grid-template-rows: minmax(160px, 220px) minmax(0, 1fr);
		}

		.viewer-tree {
			border-right: none;
			border-bottom: 1px solid color-mix(in srgb, var(--border) 64%, transparent);
		}
	}

	.file-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}

	.file-icon[data-icon="file-icons:typescript"],
	.file-icon[data-icon="file-icons:tsx"] {
		color: #3178c6;
	}

	.file-icon[data-icon="file-icons:javascript"],
	.file-icon[data-icon="file-icons:jsx"] {
		color: #f7df1e;
	}

	.file-icon[data-icon="file-icons:python"] {
		color: #3776ab;
	}

	.file-icon[data-icon="file-icons:go"] {
		color: #00add8;
	}

	.file-icon[data-icon="file-icons:go"] :global(svg) {
		width: 14px;
		height: 14px;
	}

	.file-icon[data-icon="file-icons:rust"] {
		color: #dea584;
	}

	.file-icon[data-icon="file-icons:java"] {
		color: #b07219;
	}

	.file-icon[data-icon="file-icons:kotlin"] {
		color: #7f52ff;
	}

	.file-icon[data-icon="file-icons:swift"] {
		color: #f05138;
	}

	.file-icon[data-icon="file-icons:svelte"] {
		color: #ff3e00;
	}

	.file-icon[data-icon="file-icons:vue"] {
		color: #42b883;
	}

	.file-icon[data-icon="file-icons:html"] {
		color: #e34c26;
	}

	.file-icon[data-icon="file-icons:css"] {
		color: #264de4;
	}

	.file-icon[data-icon="file-icons:json-1"],
	.file-icon[data-icon="file-icons:json-2"] {
		color: #cbcb41;
	}

	.file-icon[data-icon="file-icons:yaml"] {
		color: #cb171e;
	}

	.file-icon[data-icon="file-icons:markdown"] {
		color: #083fa1;
	}

	.file-icon[data-icon="file-icons:docker"] {
		color: #2496ed;
	}

	.file-icon[data-icon="file-icons:git"] {
		color: #f05032;
	}

	.file-icon[data-icon="file-icons:npm"] {
		color: #cb3837;
	}

	.file-icon[data-icon="file-icons:ruby"] {
		color: #cc342d;
	}

	.file-icon[data-icon="file-icons:php"] {
		color: #777bb4;
	}

	.file-icon[data-icon="file-icons:c"] {
		color: #a8b9cc;
	}

	.file-icon[data-icon="file-icons:cpp"] {
		color: #00599c;
	}

	.file-icon[data-icon="file-icons:csharp"] {
		color: #512bd4;
	}

	.file-icon[data-icon="file-icons:scala"] {
		color: #dc322f;
	}

	.file-icon[data-icon="file-icons:elixir"] {
		color: #4e2a8e;
	}

	.file-icon[data-icon="file-icons:erlang"] {
		color: #a90533;
	}

	.file-icon[data-icon="file-icons:haskell"] {
		color: #5e5185;
	}

	.file-icon[data-icon="file-icons:clojure"] {
		color: #5881d8;
	}

	.file-icon[data-icon="file-icons:dart"] {
		color: #0175c2;
	}

	.file-icon[data-icon="file-icons:lua"] {
		color: #000080;
	}

	.file-icon[data-icon="file-icons:perl"] {
		color: #39457e;
	}

	.file-icon[data-icon="file-icons:r"] {
		color: #276dc3;
	}

	.file-icon[data-icon="file-icons:sql"],
	.file-icon[data-icon="file-icons:database"] {
		color: #e38c00;
	}

	.file-icon[data-icon="file-icons:graphql"] {
		color: #e535ab;
	}

	.file-icon[data-icon="file-icons:solidity"] {
		color: #363636;
	}

	.file-icon[data-icon="file-icons:terminal"],
	.file-icon[data-icon="file-icons:bash"] {
		color: #4eaa25;
	}

	.file-icon[data-icon="file-icons:powershell"] {
		color: #5391c5;
	}

	.file-icon[data-icon="file-icons:vim"] {
		color: #019733;
	}

	.file-icon[data-icon="file-icons:ocaml"] {
		color: #ee7a16;
	}

	.file-icon[data-icon="file-icons:fsharp"] {
		color: #378bba;
	}

	.file-icon[data-icon="file-icons:elm"] {
		color: #60b5cc;
	}

	.file-icon[data-icon="file-icons:wasm"] {
		color: #654ff0;
	}

	.file-icon[data-icon="file-icons:image"] {
		color: #a074c4;
	}

	.file-icon[data-icon="file-icons:config"] {
		color: #6d8086;
	}

	.file-icon[data-icon="file-icons:lock"] {
		color: #8b8b8b;
	}

	.file-icon[data-icon="file-icons:license"] {
		color: #d4aa00;
	}
</style>
