<script lang="ts">
	import { untrack } from 'svelte';
	import Icon from '@iconify/svelte';
	import {
		ChevronDown,
		ChevronLeft,
		ChevronRight,
		Columns2,
		Edit3,
		Eye,
		FileCode,
		FolderTree,
		GitBranch,
		GitMerge,
		GitPullRequest,
		LoaderCircle,
		Minus,
		PanelLeftClose,
		PanelLeftOpen,
		Plus,
		Rows2,
		BookOpen,
		Save,
		Search,
		X,
	} from '@lucide/svelte';
	import type {
		RepoDiffFileSummary,
		RepoDiffSummary,
		RepoFileContent,
		RepoFileDiff,
		RepoFileSearchResult,
		Workspace,
	} from '../../types';
	import {
		fetchRepoDiffSummary,
		fetchRepoFileDiff,
		startRepoStatusWatch,
		stopRepoStatusWatch,
	} from '../../api/repo-diff';
	import {
		readWorkspaceRepoFile,
		readWorkspaceRepoFileAtRef,
		searchWorkspaceRepoFiles,
		writeWorkspaceRepoFile,
		invalidateRepoFileContent,
		clearFileContentCache,
		listRepoDirectory,
		invalidateRepoDirCache,
		clearDirListCache,
		type RepoDirectoryEntry,
	} from '../../api/repo-files';
	import { resolveMarkdownImages, clearImageCache } from '../../markdownImages';
	import { buildSummaryLocalCacheKey, repoDiffCache } from '../../cache/repoDiffCache';
	import { subscribeRepoDiffEvent } from '../../repoDiffService';
	import { EVENT_REPO_DIFF_LOCAL_SUMMARY } from '../../events';
	import { refreshWorkspacesStatus } from '../../state';
	import {
		buildDocumentViewerTree,
		buildDocumentViewerTreeFromDirs,
		buildExpandedKeysForQuery,
		computeChildCounts,
		computeDirChildCounts,
		dirEntriesKey,
		shouldReplaceExpandedNodeSet,
		type DocumentViewerTreeNode,
	} from '../document-viewer/tree';
	import { getDocumentViewerFileIcon } from '../document-viewer/fileIcons';
	import { renderMarkdownDocument, type DocumentRenderResult } from '../../documentRender';
	import { calculateMermaidOverlayFit } from '../document-viewer/mermaidOverlay';
	import ResizablePanel from '../ui/ResizablePanel.svelte';
	import CodeEditor from '../editor/CodeEditor.svelte';
	import CodeDiffView from '../editor/CodeDiffView.svelte';
	import PrCreateDrawer from './PrCreateDrawer.svelte';
	import LocalMergeDrawer from './LocalMergeDrawer.svelte';
	import PrLifecycleDrawer from './PrLifecycleDrawer.svelte';

	interface Props {
		workspace: Workspace | null;
	}

	const { workspace }: Props = $props();

	// ── File tree state (from DocumentViewer pattern) ────────
	type RepoFileState =
		| { status: 'idle' }
		| { status: 'loading' }
		| { status: 'loaded'; files: RepoFileSearchResult[] }
		| { status: 'error'; message: string };

	let repoFileStates = $state<Map<string, RepoFileState>>(new Map());
	let dirEntries = $state<Map<string, RepoDirectoryEntry[]>>(new Map());
	let expandedNodes = $state<Set<string>>(new Set());
	let searchQuery = $state('');
	let showFileTree = $state(true);

	// ── Diff state (per-repo) ───────────────────────────────
	let repoDiffMap = $state<Map<string, RepoDiffSummary>>(new Map());

	// ── Selection state ─────────────────────────────────────
	let selectedRepoId: string | null = $state(null);
	let selectedFilePath: string | null = $state(null);

	// ── File content state ──────────────────────────────────
	let fileDiffContent: RepoFileDiff | null = $state(null);
	let fileDiffLoading = $state(false);
	let fileDiffError: string | null = $state(null);
	let fileDiffRequestId = 0;

	// Full file contents for diff view (original = HEAD, modified = working tree)
	let originalFileContent: string | null = $state(null);
	let modifiedFileContent: string | null = $state(null);
	let fullDiffLoading = $state(false);
	let fullDiffRequestId = 0;

	let fileContent: RepoFileContent | null = $state(null);
	let fileContentLoading = $state(false);
	let fileContentRequestId = 0;

	// ── Markdown rendering state ────────────────────────────
	let renderedMarkdown: DocumentRenderResult | null = $state(null);
	let renderLoading = $state(false);
	let renderToken = 0;

	// ── Mermaid overlay state ────────────────────────────────
	let mermaidOverlayOpen = $state(false);
	let mermaidOverlayMarkup = $state('');
	let mermaidZoom = $state(1);
	let mermaidOffsetX = $state(0);
	let mermaidOffsetY = $state(0);
	let mermaidFitScale = $state(1);
	let mermaidDragging = $state(false);
	let mermaidDragPointerId = $state<number | null>(null);
	let mermaidDragOriginX = 0;
	let mermaidDragOriginY = 0;
	let mermaidDragStartOffsetX = 0;
	let mermaidDragStartOffsetY = 0;
	let mermaidCanvasEl: HTMLElement | null = null;
	let mermaidStageEl: HTMLElement | null = null;
	let mermaidIntrinsicW = $state(0);
	let mermaidIntrinsicH = $state(0);

	const openMermaidOverlay = (svgMarkup: string): void => {
		if (!svgMarkup) return;
		mermaidOverlayMarkup = svgMarkup;
		mermaidZoom = 1;
		mermaidFitScale = 1;
		mermaidOffsetX = 0;
		mermaidOffsetY = 0;
		mermaidDragging = false;
		mermaidIntrinsicW = 0;
		mermaidIntrinsicH = 0;
		mermaidOverlayOpen = true;
	};
	const closeMermaidOverlay = (): void => {
		mermaidOverlayOpen = false;
		mermaidOverlayMarkup = '';
	};
	const adjustMermaidZoom = (delta: number): void => {
		mermaidZoom = Math.min(2.5, Math.max(0.5, Math.round((mermaidZoom + delta) * 100) / 100));
	};
	const resetMermaidZoom = (): void => {
		mermaidZoom = 1;
		mermaidOffsetX = 0;
		mermaidOffsetY = 0;
	};
	const updateMermaidFit = (): void => {
		if (!mermaidCanvasEl || !mermaidStageEl) return;
		const svg = mermaidStageEl.querySelector('svg');
		if (!(svg instanceof SVGElement)) return;
		const fit = calculateMermaidOverlayFit(
			svg,
			mermaidCanvasEl.clientWidth - 32,
			mermaidCanvasEl.clientHeight - 32,
		);
		if (!fit) return;
		mermaidIntrinsicW = fit.intrinsicWidth;
		mermaidIntrinsicH = fit.intrinsicHeight;
		mermaidFitScale = fit.fitScale;
	};
	const handleMermaidPointerDown = (event: PointerEvent): void => {
		const target = event.target;
		if (!(target instanceof Element) || !target.closest('svg')) return;
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
	};
	const handleMarkdownClick = (event: MouseEvent): void => {
		const target = event.target;
		if (!(target instanceof Element)) return;
		const diagram = target.closest('.ws-mermaid-diagram');
		if (!(diagram instanceof HTMLElement)) return;
		const svg = diagram.querySelector('svg');
		if (!(svg instanceof SVGElement)) return;
		openMermaidOverlay(svg.outerHTML);
	};

	// ── View options ────────────────────────────────────────
	let unifiedDiff = $state(true);
	let editMode = $state(false);
	let previewMode = $state(false);
	let editedContent = $state<string | null>(null);
	let saving = $state(false);

	const handleContentChange = (content: string): void => {
		editedContent = content;
	};

	const saveFile = async (): Promise<void> => {
		if (!editMode || saving || editedContent === null) return;
		if (!wsId || !selectedRepoId || !selectedFilePath) return;
		saving = true;
		try {
			await writeWorkspaceRepoFile(wsId, selectedRepoId, selectedFilePath, editedContent);
			// Update local state to reflect saved content
			if (fileContent) {
				fileContent = { ...fileContent, content: editedContent };
			}
			editedContent = null;
		} catch {
			// Save failed — content stays dirty
		} finally {
			saving = false;
		}
	};

	const handleKeydown = (event: KeyboardEvent): void => {
		if ((event.metaKey || event.ctrlKey) && event.key === 's' && editMode) {
			event.preventDefault();
			void saveFile();
		}
	};

	// ── Drawer state ────────────────────────────────────────
	let drawerMode: 'none' | 'pr-create' | 'local-merge' | 'pr-lifecycle' = $state('none');
	const closeDrawer = (): void => void (drawerMode = 'none');

	// ── Derived ─────────────────────────────────────────────
	const wsId = $derived(workspace?.id ?? '');
	const repos = $derived.by(() => workspace?.repos.map((r) => ({ id: r.id, name: r.name })) ?? []);
	const selectedRepo = $derived.by(
		() => workspace?.repos.find((r) => r.id === selectedRepoId) ?? null,
	);

	// File tree data
	const repoFiles = $derived.by<RepoFileSearchResult[]>(() => {
		const all: RepoFileSearchResult[] = [];
		for (const state of repoFileStates.values()) {
			if (state.status === 'loaded') all.push(...state.files);
		}
		return all;
	});
	const filteredRepoFiles = $derived.by<RepoFileSearchResult[]>(() => {
		const query = searchQuery.trim().toLowerCase();
		if (query.length === 0) return repoFiles;
		return repoFiles.filter((result) => {
			const fileName = result.path.split('/').pop()?.toLowerCase() ?? '';
			return fileName.includes(query) || result.path.toLowerCase().includes(query);
		});
	});
	const isSearchActive = $derived(searchQuery.trim().length > 0);
	const treeNodes = $derived.by<DocumentViewerTreeNode[]>(() =>
		isSearchActive
			? buildDocumentViewerTree(repos, filteredRepoFiles, expandedNodes)
			: buildDocumentViewerTreeFromDirs(repos, dirEntries, expandedNodes),
	);
	const childCounts = $derived.by(() =>
		isSearchActive ? computeChildCounts(filteredRepoFiles) : computeDirChildCounts(dirEntries),
	);

	// Changed files set: "repoId:path" for quick lookup
	const changedFileSet = $derived.by(() => {
		const set = new Set<string>();
		for (const [repoId, summary] of repoDiffMap) {
			for (const file of summary.files) {
				set.add(`${repoId}:${file.path}`);
			}
		}
		return set;
	});

	// Get diff info for the currently selected file
	const selectedDiffFile = $derived.by((): RepoDiffFileSummary | null => {
		if (!selectedRepoId || !selectedFilePath) return null;
		const summary = repoDiffMap.get(selectedRepoId);
		return summary?.files.find((f) => f.path === selectedFilePath) ?? null;
	});
	const isChangedFile = $derived(selectedDiffFile != null);
	const isMarkdownPath = (path: string | null): boolean => {
		if (!path) return false;
		const ext = path.split('.').pop()?.toLowerCase() ?? '';
		return ['md', 'markdown', 'mdx', 'mdown'].includes(ext);
	};
	const isMarkdownFile = $derived.by(
		() => fileContent?.isMarkdown === true || isMarkdownPath(selectedFilePath),
	);
	const showRenderedMarkdown = $derived(isMarkdownFile && !editMode && !isChangedFile);
	const showDiffPreview = $derived(
		isChangedFile && previewMode && isMarkdownPath(selectedFilePath),
	);

	// Changed files for current repo (for prev/next navigation)
	const currentRepoChangedFiles = $derived.by((): RepoDiffFileSummary[] => {
		if (!selectedRepoId) return [];
		return repoDiffMap.get(selectedRepoId)?.files ?? [];
	});
	const selectedChangedFileIdx = $derived.by(() => {
		if (!selectedFilePath || !selectedDiffFile) return -1;
		return currentRepoChangedFiles.findIndex((f) => f.path === selectedFilePath);
	});

	// Repo change stats for badges
	const repoChangeStats = $derived.by(() => {
		const map = new Map<string, { added: number; removed: number; count: number }>();
		for (const [repoId, summary] of repoDiffMap) {
			map.set(repoId, {
				added: summary.totalAdded,
				removed: summary.totalRemoved,
				count: summary.files.length,
			});
		}
		return map;
	});

	// Directory change indicators: which dirs contain changed files
	const changedDirSet = $derived.by(() => {
		const set = new Set<string>();
		for (const [repoId, summary] of repoDiffMap) {
			for (const file of summary.files) {
				const parts = file.path.split('/');
				for (let i = 1; i < parts.length; i++) {
					set.add(`dir:${repoId}:${parts.slice(0, i).join('/')}`);
				}
			}
		}
		return set;
	});

	// Per-directory change counts
	const dirChangeCount = $derived.by(() => {
		const map = new Map<string, number>();
		for (const [repoId, summary] of repoDiffMap) {
			for (const file of summary.files) {
				const parts = file.path.split('/');
				for (let i = 1; i < parts.length; i++) {
					const key = `dir:${repoId}:${parts.slice(0, i).join('/')}`;
					map.set(key, (map.get(key) ?? 0) + 1);
				}
			}
		}
		return map;
	});

	// Repo PR state lookup
	const getRepoPrState = (repoId: string): 'none' | 'open' | 'merged' | 'draft' => {
		const repo = workspace?.repos.find((r) => r.id === repoId);
		const tracked = repo?.trackedPullRequest;
		if (!tracked) return 'none';
		const state = tracked.state.toLowerCase();
		if (tracked.merged || state === 'merged') return 'merged';
		if (tracked.draft) return 'draft';
		if (state === 'open') return 'open';
		return 'none';
	};

	// ── File tree actions ───────────────────────────────────

	/** Load the full file index for search (deferred until user types). */
	const loadRepoFiles = (workspaceId: string, repoId: string): void => {
		const current = repoFileStates.get(repoId);
		if (current?.status === 'loaded' || current?.status === 'loading') return;
		repoFileStates = new Map(repoFileStates).set(repoId, { status: 'loading' });
		void searchWorkspaceRepoFiles(workspaceId, '', 5000, repoId)
			.then((files) => {
				repoFileStates = new Map(repoFileStates).set(repoId, { status: 'loaded', files });
			})
			.catch((err) => {
				repoFileStates = new Map(repoFileStates).set(repoId, {
					status: 'error',
					message: err instanceof Error ? err.message : 'Failed to load files.',
				});
			});
	};

	/** Load directory entries for lazy tree browsing. */
	const loadDirEntries = (workspaceId: string, repoId: string, dirPath: string): void => {
		const key = dirEntriesKey(repoId, dirPath);
		if (dirEntries.has(key)) return; // already loaded
		void listRepoDirectory(workspaceId, repoId, dirPath)
			.then((entries) => {
				dirEntries = new Map(dirEntries).set(key, entries);
			})
			.catch(() => {
				// Silently fail — the directory just won't show children
			});
	};

	const loadRepoDiff = async (
		workspaceId: string,
		repoId: string,
		force = false,
	): Promise<void> => {
		if (!force && repoDiffMap.has(repoId)) return;
		const cacheKey = buildSummaryLocalCacheKey(workspaceId, repoId);
		const cached = repoDiffCache.getSummary(cacheKey);
		if (!force && cached) {
			repoDiffMap = new Map(repoDiffMap).set(repoId, cached.value);
			if (!cached.stale) return;
		}
		try {
			const fetched = await fetchRepoDiffSummary(workspaceId, repoId);
			repoDiffMap = new Map(repoDiffMap).set(repoId, fetched);
			repoDiffCache.setSummary(cacheKey, fetched);
		} catch {
			// keep cached if available
		}
	};

	// Track repos with active file watchers
	const activeWatchers = new Set<string>();

	const startWatcherForRepo = (workspaceId: string, repoId: string): void => {
		if (activeWatchers.has(repoId)) return;
		activeWatchers.add(repoId);
		void startRepoStatusWatch(workspaceId, repoId).catch(() => {
			activeWatchers.delete(repoId);
		});
	};

	const stopAllWatchers = (): void => {
		for (const repoId of activeWatchers) {
			void stopRepoStatusWatch(wsId, repoId).catch(() => {});
		}
		activeWatchers.clear();
	};

	// Refresh the currently viewed file content
	const refreshCurrentFile = (): void => {
		const currentWsId = wsId;
		const repoId = selectedRepoId;
		const path = selectedFilePath;
		if (!currentWsId || !repoId || !path) return;
		const diffFile = selectedDiffFile;
		if (diffFile) {
			void loadFileDiff(currentWsId, repoId, diffFile);
		} else {
			void loadFileContent(currentWsId, repoId, path);
		}
	};

	const stopWatcherForRepo = (repoId: string): void => {
		if (!activeWatchers.has(repoId)) return;
		activeWatchers.delete(repoId);
		void stopRepoStatusWatch(wsId, repoId).catch(() => {});
	};

	const toggleNode = (key: string): void => {
		const next = new Set(expandedNodes);
		if (next.has(key)) {
			next.delete(key);
			// Evict diff data and stop watcher for collapsed repos
			if (key.startsWith('repo:')) {
				const repoId = key.slice(5);
				if (repoDiffMap.has(repoId)) {
					const nextMap = new Map(repoDiffMap);
					nextMap.delete(repoId);
					repoDiffMap = nextMap;
				}
				stopWatcherForRepo(repoId);
			}
		} else {
			next.add(key);
			if (key.startsWith('repo:') && wsId) {
				const repoId = key.slice(5);
				selectedRepoId = repoId;
				// Lazy-load root directory entries for browsing
				loadDirEntries(wsId, repoId, '');
				void loadRepoDiff(wsId, repoId);
				startWatcherForRepo(wsId, repoId);
			} else if (key.startsWith('dir:') && wsId) {
				// Lazy-load directory children on expand
				const rest = key.slice(4); // "repoId:path"
				const colonIdx = rest.indexOf(':');
				if (colonIdx > 0) {
					const repoId = rest.slice(0, colonIdx);
					const dirPath = rest.slice(colonIdx + 1);
					loadDirEntries(wsId, repoId, dirPath);
				}
			}
		}
		expandedNodes = next;
	};

	const selectTreeFile = (path: string, repoId: string): void => {
		const sameFile = selectedRepoId === repoId && selectedFilePath === path;
		selectedRepoId = repoId;
		selectedFilePath = path;
		editMode = false;
		previewMode = false;
		editedContent = null;
		if (!sameFile) {
			fileDiffContent = null;
			originalFileContent = null;
			modifiedFileContent = null;
			fileContent = null;
			fileDiffError = null;
			renderedMarkdown = null;
		}
		renderToken += 1;
		fileDiffRequestId += 1;
		fileContentRequestId += 1;
	};

	const navigateChangedFile = (delta: number): void => {
		const files = currentRepoChangedFiles;
		if (files.length === 0 || selectedChangedFileIdx < 0) return;
		const next = Math.max(0, Math.min(files.length - 1, selectedChangedFileIdx + delta));
		if (next !== selectedChangedFileIdx) {
			selectTreeFile(files[next].path, selectedRepoId!);
		}
	};

	// ── File content loading ────────────────────────────────
	const loadFileDiff = async (
		wsId: string,
		repoId: string,
		file: RepoDiffFileSummary,
	): Promise<void> => {
		const requestId = ++fileDiffRequestId;
		fileDiffLoading = true;
		fileDiffError = null;
		try {
			const fetched = await fetchRepoFileDiff(
				wsId,
				repoId,
				file.path,
				file.prevPath ?? '',
				file.status ?? '',
			);
			if (requestId !== fileDiffRequestId) return;
			fileDiffContent = fetched;
		} catch (err) {
			if (requestId !== fileDiffRequestId) return;
			fileDiffError = err instanceof Error ? err.message : 'Failed to load diff';
			fileDiffContent = null;
		} finally {
			if (requestId === fileDiffRequestId) fileDiffLoading = false;
		}
	};

	const loadFullDiffContents = async (
		wsId: string,
		repoId: string,
		path: string,
	): Promise<void> => {
		const requestId = ++fullDiffRequestId;
		fullDiffLoading = true;
		originalFileContent = null;
		modifiedFileContent = null;
		try {
			const [origResult, modResult] = await Promise.all([
				readWorkspaceRepoFileAtRef(wsId, repoId, path, 'HEAD'),
				readWorkspaceRepoFile(wsId, repoId, path),
			]);
			if (requestId !== fullDiffRequestId) return;
			originalFileContent = origResult.found ? origResult.content : '';
			modifiedFileContent = modResult.content;
		} catch {
			if (requestId !== fullDiffRequestId) return;
			// Fall back to patch-only mode
			originalFileContent = null;
			modifiedFileContent = null;
		} finally {
			if (requestId === fullDiffRequestId) fullDiffLoading = false;
		}
	};

	const loadFileContent = async (wsId: string, repoId: string, path: string): Promise<void> => {
		const requestId = ++fileContentRequestId;
		fileContentLoading = true;
		try {
			const content = await readWorkspaceRepoFile(wsId, repoId, path);
			if (requestId !== fileContentRequestId) return;
			fileContent = content;
		} catch {
			if (requestId !== fileContentRequestId) return;
			fileContent = null;
		} finally {
			if (requestId === fileContentRequestId) fileContentLoading = false;
		}
	};

	// ── Effects ─────────────────────────────────────────────

	// Load full file index for all expanded repos when search starts
	$effect(() => {
		const query = searchQuery.trim();
		if (query.length === 0) return;
		const currentWsId = wsId;
		if (!currentWsId) return;
		for (const key of expandedNodes) {
			if (key.startsWith('repo:')) {
				loadRepoFiles(currentWsId, key.slice(5));
			}
		}
	});

	// Auto-expand search results
	$effect(() => {
		const query = searchQuery.trim().toLowerCase();
		if (query.length === 0) return;
		const toExpand = buildExpandedKeysForQuery(filteredRepoFiles);
		const next = new Set(expandedNodes);
		for (const key of toExpand) next.add(key);
		if (shouldReplaceExpandedNodeSet(expandedNodes, next)) {
			expandedNodes = next;
		}
	});

	// Reset only when workspace ID changes (not on every reference update from polling)
	let lastWorkspaceId = '';
	$effect(() => {
		const ws = workspace;
		if (!ws) return;
		if (ws.id === lastWorkspaceId) return;
		lastWorkspaceId = ws.id;
		repoFileStates = new Map();
		dirEntries = new Map();
		repoDiffMap = new Map();
		selectedRepoId = null;
		selectedFilePath = null;
		fileDiffContent = null;
		fileContent = null;
		expandedNodes = new Set();
		searchQuery = '';
		clearImageCache();
		clearFileContentCache();
		clearDirListCache();
		// Auto-expand first repo
		const firstRepo = ws.repos[0];
		if (firstRepo) {
			expandedNodes = new Set([`repo:${firstRepo.id}`]);
			untrack(() => {
				loadDirEntries(ws.id, firstRepo.id, '');
				void loadRepoDiff(ws.id, firstRepo.id);
			});
		}
	});

	// Load file content or diff when selection changes
	$effect(() => {
		const currentWsId = wsId;
		const repoId = selectedRepoId;
		const path = selectedFilePath;
		const diffFile = selectedDiffFile;
		const editing = editMode;
		if (!currentWsId || !repoId || !path) return;
		// Load file content for edit mode or preview mode
		if (editing || previewMode) {
			if (!fileContent || fileContent.path !== path) {
				void loadFileContent(currentWsId, repoId, path);
			}
			// Also load diff if we have a changed file (for when preview is toggled off)
			if (diffFile && !fileDiffContent) {
				void loadFileDiff(currentWsId, repoId, diffFile);
			}
			return;
		}
		if (diffFile) {
			void loadFullDiffContents(currentWsId, repoId, path);
			void loadFileDiff(currentWsId, repoId, diffFile);
		} else {
			void loadFileContent(currentWsId, repoId, path);
		}
	});

	// Render markdown when file content loads (for unchanged files or diff preview)
	$effect(() => {
		const content = fileContent;
		const shouldRender = showRenderedMarkdown || showDiffPreview;
		if (!content || !shouldRender) {
			renderedMarkdown = null;
			return;
		}
		const token = ++renderToken;
		renderLoading = true;
		void renderMarkdownDocument(content.content)
			.then(async (result) => {
				if (token !== renderToken) return;
				if (selectedRepoId && selectedFilePath) {
					try {
						result = {
							...result,
							html: await resolveMarkdownImages(result.html, {
								workspaceId: wsId,
								repoId: selectedRepoId,
								markdownFilePath: selectedFilePath,
							}),
						};
					} catch {
						// Show markdown without images on failure
					}
				}
				if (token !== renderToken) return;
				renderedMarkdown = result;
				renderLoading = false;
			})
			.catch(() => {
				if (token !== renderToken) return;
				renderedMarkdown = null;
				renderLoading = false;
			});
	});

	// Update mermaid overlay fit when opened
	$effect(() => {
		if (mermaidOverlayOpen && mermaidOverlayMarkup) {
			// Wait for DOM to render the SVG
			queueMicrotask(updateMermaidFit);
		}
	});

	// Subscribe to repo diff events — refresh data when files change on disk
	$effect(() => {
		const currentWsId = wsId;
		if (!currentWsId) return;
		const unsub = subscribeRepoDiffEvent<{
			workspaceId: string;
			repoId: string;
			summary: import('../../types').RepoDiffSummary;
		}>(EVENT_REPO_DIFF_LOCAL_SUMMARY, (payload) => {
			if (payload.workspaceId !== currentWsId) return;
			// Update the diff summary for this repo
			repoDiffMap = new Map(repoDiffMap).set(payload.repoId, payload.summary);
			repoDiffCache.setSummary(
				buildSummaryLocalCacheKey(currentWsId, payload.repoId),
				payload.summary,
			);
			// Invalidate cached file/dir content for this repo (files changed on disk)
			invalidateRepoFileContent(currentWsId, payload.repoId);
			invalidateRepoDirCache(currentWsId, payload.repoId);
			// If we're viewing a file in this repo, refresh it
			if (payload.repoId === selectedRepoId && selectedFilePath) {
				refreshCurrentFile();
			}
		});
		return unsub;
	});

	// Start watcher for initially expanded repo
	$effect(() => {
		const currentWsId = wsId;
		if (!currentWsId) return;
		for (const key of expandedNodes) {
			if (key.startsWith('repo:')) {
				startWatcherForRepo(currentWsId, key.slice(5));
			}
		}
	});

	// Cleanup watchers on unmount
	$effect(() => {
		return () => stopAllWatchers();
	});

	// Check if file is changed helper
	const isFileChanged = (repoId: string, path: string): boolean =>
		changedFileSet.has(`${repoId}:${path}`);

	const getFileDiffInfo = (repoId: string, path: string): RepoDiffFileSummary | undefined => {
		const summary = repoDiffMap.get(repoId);
		return summary?.files.find((f) => f.path === path);
	};
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="urv">
	{#if !workspace}
		<div class="urv-empty">
			<FolderTree size={48} />
			<p>Select a thread to view files</p>
		</div>
	{:else if workspace.repos.length === 0}
		<div class="urv-empty">
			<FolderTree size={48} />
			<p>No repositories in this workspace</p>
		</div>
	{:else}
		{#snippet sidebarPane()}
			<aside class="urv-sidebar">
				<div class="urv-tree-header">
					<div class="urv-tree-title">
						<FolderTree size={12} />
						<span>Files</span>
					</div>
					<div class="urv-tree-actions">
						{#if selectedRepo}
							{@const tracked = selectedRepo.trackedPullRequest}
							{#if tracked && tracked.state.toLowerCase() === 'open'}
								<button
									type="button"
									class="urv-tree-action urv-action-pr"
									title="View Pull Request"
									onclick={() => (drawerMode = 'pr-lifecycle')}
								>
									<GitPullRequest size={11} />
								</button>
							{:else}
								<button
									type="button"
									class="urv-tree-action"
									title="Create Pull Request"
									onclick={() => (drawerMode = 'pr-create')}
								>
									<GitPullRequest size={11} />
								</button>
								<button
									type="button"
									class="urv-tree-action"
									title="Local Merge"
									onclick={() => (drawerMode = 'local-merge')}
								>
									<GitMerge size={11} />
								</button>
							{/if}
						{/if}
						<button
							type="button"
							class="urv-tree-action"
							title="Hide file tree"
							onclick={() => (showFileTree = false)}
						>
							<PanelLeftClose size={12} />
						</button>
					</div>
				</div>
				<div class="urv-tree-search">
					<Search size={11} />
					<input
						type="text"
						placeholder="Filter files..."
						value={searchQuery}
						oninput={(e) => (searchQuery = (e.currentTarget as HTMLInputElement).value)}
					/>
				</div>
				<div class="urv-tree-list">
					{#each treeNodes as node (node.key)}
						{#if node.kind === 'repo'}
							{@const stats = repoChangeStats.get(node.repoId)}
							{@const prState = getRepoPrState(node.repoId)}
							<button
								type="button"
								class="urv-tree-repo"
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
								<span class="urv-tree-label">{node.label}</span>
								{#if prState === 'open'}
									<button
										type="button"
										class="urv-pr-indicator urv-pr-open"
										title="View Pull Request"
										onclick={(e) => {
											e.stopPropagation();
											selectedRepoId = node.repoId;
											drawerMode = 'pr-lifecycle';
										}}
									>
										<GitPullRequest size={10} />
									</button>
								{:else if prState === 'draft'}
									<button
										type="button"
										class="urv-pr-indicator urv-pr-draft"
										title="View Draft PR"
										onclick={(e) => {
											e.stopPropagation();
											selectedRepoId = node.repoId;
											drawerMode = 'pr-lifecycle';
										}}
									>
										<GitPullRequest size={10} />
									</button>
								{/if}
								{#if stats && stats.count > 0}
									<span class="urv-tree-change-badge">
										<span class="urv-badge-add">+{stats.added}</span>
										<span class="urv-badge-del">-{stats.removed}</span>
									</span>
								{:else if childCounts.has(node.key)}
									<span class="urv-tree-count">{childCounts.get(node.key)}</span>
								{/if}
							</button>
							{#if expandedNodes.has(node.key)}
								{@const repoState = repoFileStates.get(node.repoId)}
								{#if repoState?.status === 'loading'}
									<div class="urv-tree-state" style="--depth:1;">
										<span class="spin"><LoaderCircle size={14} /></span>
										<span>Loading...</span>
									</div>
								{:else if repoState?.status === 'error'}
									<div class="urv-tree-state error" style="--depth:1;">
										{repoState.message}
									</div>
								{/if}
							{/if}
						{:else if node.kind === 'dir'}
							{@const dirChanged = changedDirSet.has(node.key)}
							{@const dirChanges = dirChangeCount.get(node.key) ?? 0}
							<button
								type="button"
								class="urv-tree-dir"
								class:expanded={expandedNodes.has(node.key)}
								class:has-changes={dirChanged}
								style={`--depth:${node.depth};`}
								onclick={() => toggleNode(node.key)}
							>
								{#if expandedNodes.has(node.key)}
									<ChevronDown size={11} />
								{:else}
									<ChevronRight size={11} />
								{/if}
								<span class="urv-tree-label">{node.label}</span>
								{#if dirChanged && dirChanges > 0}
									<span class="urv-tree-dir-changes">{dirChanges}</span>
								{:else if childCounts.has(node.key)}
									<span class="urv-tree-count">{childCounts.get(node.key)}</span>
								{/if}
							</button>
						{:else}
							{@const changed = isFileChanged(node.repoId, node.path)}
							{@const diffInfo = changed ? getFileDiffInfo(node.repoId, node.path) : undefined}
							<button
								type="button"
								class="urv-tree-file"
								class:selected={node.path === selectedFilePath && node.repoId === selectedRepoId}
								class:changed
								style={`--depth:${node.depth};`}
								title={node.path}
								onclick={() => selectTreeFile(node.path, node.repoId)}
							>
								<span class="urv-file-icon" data-icon={getDocumentViewerFileIcon(node.path)}>
									<Icon icon={getDocumentViewerFileIcon(node.path)} width="12" />
								</span>
								<span class="urv-tree-file-name">{node.label}</span>
								{#if diffInfo}
									<span class="urv-tree-file-diff">
										{#if diffInfo.added > 0}<span class="urv-badge-add">+{diffInfo.added}</span
											>{/if}
										{#if diffInfo.removed > 0}<span class="urv-badge-del">-{diffInfo.removed}</span
											>{/if}
									</span>
								{/if}
							</button>
						{/if}
					{/each}
				</div>
			</aside>
		{/snippet}

		{#snippet mainPanel()}
			<main class="urv-main">
				{#if !showFileTree}
					<button
						type="button"
						class="urv-show-tree-btn"
						aria-label="Show file tree"
						onclick={() => (showFileTree = true)}
					>
						<PanelLeftOpen size={14} />
					</button>
				{/if}
				{#if selectedFilePath}
					<div class="urv-file-header">
						<div class="urv-file-info">
							<span class="urv-file-path">{selectedFilePath}</span>
							{#if selectedDiffFile}
								<span class="urv-file-stats">
									{#if selectedDiffFile.added > 0}<span class="urv-stat-add"
											>+{selectedDiffFile.added}</span
										>{/if}
									{#if selectedDiffFile.removed > 0}<span class="urv-stat-del"
											>-{selectedDiffFile.removed}</span
										>{/if}
								</span>
							{/if}
						</div>
						<div class="urv-file-actions">
							{#if isChangedFile && currentRepoChangedFiles.length > 1}
								<div class="urv-file-nav">
									<button
										type="button"
										class="urv-nav-btn"
										disabled={selectedChangedFileIdx <= 0}
										aria-label="Previous changed file"
										onclick={() => navigateChangedFile(-1)}
									>
										<ChevronLeft size={14} />
									</button>
									<span class="urv-nav-pos"
										>{selectedChangedFileIdx + 1}/{currentRepoChangedFiles.length}</span
									>
									<button
										type="button"
										class="urv-nav-btn"
										disabled={selectedChangedFileIdx >= currentRepoChangedFiles.length - 1}
										aria-label="Next changed file"
										onclick={() => navigateChangedFile(1)}
									>
										<ChevronRight size={14} />
									</button>
								</div>
							{/if}
							{#if isChangedFile && !editMode && !previewMode}
								<button
									type="button"
									class="urv-toggle-btn"
									class:active={!unifiedDiff}
									aria-label={unifiedDiff ? 'Split view' : 'Unified view'}
									onclick={() => (unifiedDiff = !unifiedDiff)}
								>
									{#if unifiedDiff}<Columns2 size={13} />{:else}<Rows2 size={13} />{/if}
								</button>
							{/if}
							{#if isChangedFile && isMarkdownPath(selectedFilePath)}
								<button
									type="button"
									class="urv-toggle-btn"
									class:active={previewMode}
									aria-label={previewMode ? 'Back to diff' : 'Preview rendered'}
									title={previewMode ? 'Back to diff' : 'Preview rendered markdown'}
									onclick={() => {
										previewMode = !previewMode;
									}}
								>
									<BookOpen size={13} />
								</button>
							{/if}
							{#if editMode && editedContent !== null}
								<button
									type="button"
									class="urv-save-btn"
									title="Save changes (⌘S)"
									disabled={saving}
									onclick={() => void saveFile()}
								>
									<Save size={13} />
								</button>
							{/if}
							<button
								type="button"
								class="urv-toggle-btn"
								class:active={editMode}
								aria-label={editMode ? 'Back to view' : 'Edit file'}
								title={editMode ? 'Back to view' : 'Edit file'}
								onclick={() => {
									editMode = !editMode;
									editedContent = null;
								}}
							>
								{#if editMode}<Eye size={13} />{:else}<Edit3 size={13} />{/if}
							</button>
						</div>
					</div>
					<div class="urv-editor">
						{#if editMode && fileContent}
							<CodeEditor
								content={fileContent.content}
								filePath={selectedFilePath}
								readOnly={false}
								onContentChange={handleContentChange}
							/>
						{:else if editMode && !fileContent && !fileContentLoading}
							<div class="urv-placeholder"><p>Loading file for editing...</p></div>
						{:else if showDiffPreview && renderedMarkdown}
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<!-- svelte-ignore a11y_click_events_have_key_events -->
							<div class="urv-markdown-view" onclick={handleMarkdownClick}>
								<!-- eslint-disable-next-line svelte/no-at-html-tags -->
								{@html renderedMarkdown.html}
							</div>
						{:else if showDiffPreview && (renderLoading || fileContentLoading)}
							<div class="urv-placeholder"><p>Rendering preview...</p></div>
						{:else if isChangedFile}
							<CodeDiffView
								originalContent={originalFileContent}
								modifiedContent={modifiedFileContent}
								patch={fileDiffContent?.patch ?? null}
								filePath={selectedFilePath}
								unified={unifiedDiff}
								loading={fileDiffLoading || fullDiffLoading}
								error={fileDiffError}
								binary={fileDiffContent?.binary ?? false}
								truncated={fileDiffContent?.truncated ?? false}
								totalLines={fileDiffContent?.totalLines ?? 0}
							/>
						{:else if showRenderedMarkdown && renderedMarkdown}
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<!-- svelte-ignore a11y_click_events_have_key_events -->
							<div class="urv-markdown-view" onclick={handleMarkdownClick}>
								<!-- eslint-disable-next-line svelte/no-at-html-tags -->
								{@html renderedMarkdown.html}
							</div>
						{:else if showRenderedMarkdown && renderLoading}
							<div class="urv-placeholder"><p>Rendering markdown...</p></div>
						{:else if fileContent}
							<CodeEditor
								content={fileContent.content}
								filePath={selectedFilePath}
								readOnly={true}
							/>
						{:else if fileContentLoading}
							<div class="urv-placeholder"><p>Loading file...</p></div>
						{:else}
							<div class="urv-placeholder">
								<FileCode size={24} />
								<p>Unable to load file</p>
							</div>
						{/if}
					</div>
				{:else}
					<div class="urv-placeholder">
						<FolderTree size={32} strokeWidth={1.5} />
						<p>Select a file to view</p>
					</div>
				{/if}
			</main>
		{/snippet}

		{#if showFileTree}
			<ResizablePanel
				direction="horizontal"
				initialRatio={0.3}
				minRatio={0.2}
				maxRatio={0.45}
				storageKey="workset:unified-repo:sidebarRatio"
			>
				{@render sidebarPane()}
				{#snippet second()}
					{@render mainPanel()}
				{/snippet}
			</ResizablePanel>
		{:else}
			{@render mainPanel()}
		{/if}
	{/if}
</div>

{#if mermaidOverlayOpen}
	<div
		class="mm-overlay"
		role="button"
		tabindex="0"
		aria-label="Close diagram"
		onclick={closeMermaidOverlay}
		onkeydown={(e) => {
			if (e.key === 'Escape') closeMermaidOverlay();
		}}
	>
		<div
			class="mm-panel"
			role="dialog"
			aria-modal="true"
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
		>
			<div class="mm-toolbar">
				<div class="mm-zoom-actions">
					<button
						type="button"
						class="mm-btn"
						aria-label="Zoom out"
						onclick={() => adjustMermaidZoom(-0.1)}><Minus size={15} /></button
					>
					<button type="button" class="mm-btn-text" onclick={resetMermaidZoom}
						>{Math.round(mermaidZoom * 100)}%</button
					>
					<button
						type="button"
						class="mm-btn"
						aria-label="Zoom in"
						onclick={() => adjustMermaidZoom(0.1)}><Plus size={15} /></button
					>
				</div>
				<button type="button" class="mm-btn" aria-label="Close" onclick={closeMermaidOverlay}
					><X size={15} /></button
				>
			</div>
			<div class="mm-canvas">
				<div
					bind:this={mermaidCanvasEl}
					class="mm-surface"
					class:dragging={mermaidDragging}
					role="presentation"
					onpointerdown={handleMermaidPointerDown}
					onpointermove={handleMermaidPointerMove}
					onpointerup={handleMermaidPointerUp}
					onpointercancel={handleMermaidPointerUp}
				>
					<div
						bind:this={mermaidStageEl}
						class="mm-stage"
						style={`--mm-scale:${mermaidFitScale * mermaidZoom}; --mm-x:${mermaidOffsetX}px; --mm-y:${mermaidOffsetY}px; --mm-w:${mermaidIntrinsicW}px; --mm-h:${mermaidIntrinsicH}px;`}
					>
						<!-- eslint-disable-next-line svelte/no-at-html-tags -->
						{@html mermaidOverlayMarkup}
					</div>
				</div>
			</div>
		</div>
	</div>
{/if}

{#if workspace && selectedRepo}
	<PrCreateDrawer
		open={drawerMode === 'pr-create'}
		workspaceId={workspace.id}
		repoId={selectedRepo.id}
		repoName={selectedRepo.name}
		branch={selectedRepo.currentBranch || 'main'}
		baseBranch={selectedRepo.defaultBranch || 'main'}
		onClose={closeDrawer}
		onCreated={() => {
			closeDrawer();
			void refreshWorkspacesStatus(true);
		}}
	/>
	<LocalMergeDrawer
		open={drawerMode === 'local-merge'}
		workspaceId={workspace.id}
		repoId={selectedRepo.id}
		repoName={selectedRepo.name}
		branch={selectedRepo.currentBranch || 'main'}
		baseBranch={selectedRepo.defaultBranch || 'main'}
		onClose={closeDrawer}
		onMerged={() => {
			void refreshWorkspacesStatus(true);
		}}
	/>
	<PrLifecycleDrawer
		open={drawerMode === 'pr-lifecycle'}
		workspaceId={workspace.id}
		repoId={selectedRepo.id}
		repoName={selectedRepo.name}
		branch={selectedRepo.currentBranch || 'main'}
		trackedPr={selectedRepo.trackedPullRequest ?? null}
		onClose={closeDrawer}
		onStatusChanged={() => void refreshWorkspacesStatus(true)}
	/>
{/if}

<style>
	.urv {
		display: flex;
		flex-direction: column;
		height: 100%;
		background: var(--bg);
	}
	.urv-empty {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 16px;
		color: var(--muted);
		opacity: 0.5;
	}

	/* ── Sidebar ─────────────────────────────────────────── */
	.urv-sidebar {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		background: var(--panel);
		border-right: 1px solid var(--border);
	}
	.urv-tree-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 8px 10px;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
	}
	.urv-tree-title {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xxs);
		font-weight: 500;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--muted);
	}
	.urv-tree-actions {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}
	.urv-tree-action {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		padding: 0;
		border: 1px solid transparent;
		border-radius: 5px;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition: all var(--transition-fast);
	}
	.urv-tree-action:hover {
		color: var(--text);
		background: var(--hover-bg);
		border-color: var(--border);
	}
	.urv-tree-search {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 40%, transparent);
		color: var(--muted);
	}
	.urv-tree-search input {
		flex: 1;
		background: transparent;
		border: none;
		color: var(--text);
		font-size: var(--text-xs);
		font-family: inherit;
		outline: none;
	}
	.urv-tree-search input::placeholder {
		color: var(--subtle);
	}
	.urv-tree-list {
		flex: 1;
		min-height: 0;
		overflow-y: auto;
	}

	/* ── Tree nodes ──────────────────────────────────────── */
	.urv-tree-repo,
	.urv-tree-dir,
	.urv-tree-file {
		display: flex;
		align-items: center;
		gap: 5px;
		width: 100%;
		padding: 4px 8px;
		padding-left: calc(8px + var(--depth, 0) * 14px);
		border: none;
		background: transparent;
		color: var(--text);
		text-align: left;
		font-size: var(--text-xxs);
		cursor: pointer;
		transition: background var(--transition-fast);
	}
	.urv-tree-repo:hover,
	.urv-tree-dir:hover,
	.urv-tree-file:hover {
		background: var(--hover-bg);
	}
	.urv-tree-repo {
		font-weight: 500;
		padding-top: 6px;
		padding-bottom: 6px;
	}
	.urv-tree-label {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex: 1;
		min-width: 0;
	}
	.urv-tree-count {
		font-size: var(--text-2xs);
		color: var(--subtle);
		font-family: var(--font-mono);
	}
	.urv-tree-change-badge {
		display: inline-flex;
		gap: 4px;
		font-size: var(--text-mono-2xs);
		font-family: var(--font-mono);
		flex-shrink: 0;
	}
	.urv-badge-add {
		color: var(--success);
	}
	.urv-badge-del {
		color: var(--danger);
	}
	.urv-tree-state {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 8px;
		padding-left: calc(8px + var(--depth, 0) * 14px);
		font-size: var(--text-2xs);
		color: var(--muted);
	}
	.urv-tree-state.error {
		color: var(--danger);
	}

	.urv-tree-file {
		color: var(--muted);
	}
	.urv-tree-file.selected {
		background: var(--active-accent-bg);
		color: var(--text);
	}
	.urv-tree-file.selected .urv-tree-file-name {
		color: var(--accent);
	}
	.urv-tree-file.changed .urv-tree-file-name {
		color: var(--text);
	}
	/* File icon colors by type (data-icon attribute) */
	.urv-file-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
	.urv-file-icon[data-icon='file-icons:typescript'],
	.urv-file-icon[data-icon='file-icons:tsx'] {
		color: #3178c6;
	}
	.urv-file-icon[data-icon='file-icons:javascript'],
	.urv-file-icon[data-icon='file-icons:jsx'] {
		color: #f7df1e;
	}
	.urv-file-icon[data-icon='file-icons:python'] {
		color: #3776ab;
	}
	.urv-file-icon[data-icon='file-icons:go'] {
		color: #00add8;
	}
	.urv-file-icon[data-icon='file-icons:go'] :global(svg) {
		width: 14px;
		height: 14px;
	}
	.urv-file-icon[data-icon='file-icons:rust'] {
		color: #dea584;
	}
	.urv-file-icon[data-icon='file-icons:java'] {
		color: #b07219;
	}
	.urv-file-icon[data-icon='file-icons:kotlin'] {
		color: #7f52ff;
	}
	.urv-file-icon[data-icon='file-icons:swift'] {
		color: #f05138;
	}
	.urv-file-icon[data-icon='file-icons:ruby'] {
		color: #cc342d;
	}
	.urv-file-icon[data-icon='file-icons:php'] {
		color: #777bb4;
	}
	.urv-file-icon[data-icon='file-icons:c'] {
		color: #a8b4be;
	}
	.urv-file-icon[data-icon='file-icons:cpp'] {
		color: #f34b7d;
	}
	.urv-file-icon[data-icon='file-icons:csharp'] {
		color: #a370c4;
	}
	.urv-file-icon[data-icon='file-icons:html'] {
		color: #e34c26;
	}
	.urv-file-icon[data-icon='file-icons:css'] {
		color: #563d7c;
	}
	.urv-file-icon[data-icon='file-icons:vue'] {
		color: #41b883;
	}
	.urv-file-icon[data-icon='file-icons:svelte'] {
		color: #ff3e00;
	}
	.urv-file-icon[data-icon='file-icons:json-1'] {
		color: #cbcb41;
	}
	.urv-file-icon[data-icon='file-icons:yaml'] {
		color: #cb171e;
	}
	.urv-file-icon[data-icon='codicon:markdown'] {
		color: #e2725b;
	}
	.urv-file-icon[data-icon='file-icons:config'] {
		color: #c4a56a;
	}
	.urv-file-icon[data-icon='file-icons:toml'] {
		color: #c89b6a;
	}
	.urv-file-icon[data-icon='file-icons:docker'] {
		color: #2496ed;
	}
	.urv-file-icon[data-icon='file-icons:git'] {
		color: #f14e32;
	}
	.urv-file-icon[data-icon='file-icons:npm'] {
		color: #cb3837;
	}
	.urv-file-icon[data-icon='file-icons:terminal'] {
		color: #4eaa25;
	}
	.urv-file-icon[data-icon='file-icons:image'] {
		color: #a074c4;
	}
	.urv-file-icon[data-icon='codicon:archive'] {
		color: #acacac;
	}
	.urv-file-icon[data-icon='codicon:lock'] {
		color: #e8d44d;
	}
	.urv-file-icon[data-icon='codicon:law'] {
		color: #d0b352;
	}
	.urv-file-icon[data-icon='file-icons:gnu'] {
		color: #e8883a;
	}
	.urv-file-icon[data-icon='file-icons:graphql'] {
		color: #e10098;
	}
	.urv-file-icon[data-icon='file-icons:proto'] {
		color: #4285f4;
	}
	.urv-file-icon[data-icon='codicon:database'] {
		color: #dad8d8;
	}
	.urv-file-icon[data-icon='codicon:table'] {
		color: #41a35d;
	}
	.urv-tree-file-name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.urv-tree-file-diff {
		display: inline-flex;
		gap: 3px;
		font-size: var(--text-mono-2xs);
		font-family: var(--font-mono);
		flex-shrink: 0;
	}

	/* Directory change indicator */
	.urv-tree-dir.has-changes .urv-tree-label {
		color: var(--text);
	}
	.urv-tree-dir-changes {
		font-size: var(--text-mono-2xs);
		font-family: var(--font-mono);
		color: var(--warning);
		background: color-mix(in srgb, var(--warning) 12%, transparent);
		padding: 0 5px;
		border-radius: 8px;
		flex-shrink: 0;
	}

	/* PR indicator on repo node */
	.urv-pr-indicator {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		padding: 0;
		border: none;
		border-radius: 4px;
		background: transparent;
		cursor: pointer;
		flex-shrink: 0;
		transition: all var(--transition-fast);
	}
	.urv-pr-open {
		color: var(--success);
	}
	.urv-pr-draft {
		color: var(--muted);
	}
	.urv-pr-indicator:hover {
		background: var(--hover-bg);
	}

	/* PR action in header — highlight when PR is tracked */
	.urv-action-pr {
		color: var(--success);
	}

	/* ── Main panel ──────────────────────────────────────── */
	.urv-main {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		min-width: 0;
		background: var(--bg);
		position: relative;
	}
	.urv-show-tree-btn {
		position: absolute;
		top: 8px;
		left: 8px;
		z-index: 5;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		padding: 0;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: var(--panel);
		color: var(--muted);
		cursor: pointer;
		transition: all var(--transition-fast);
	}
	.urv-show-tree-btn:hover {
		color: var(--text);
		background: var(--hover-bg);
	}

	.urv-file-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 8px 16px;
		background: var(--panel-strong);
		border-bottom: 1px solid var(--border);
		flex-shrink: 0;
	}
	.urv-file-info {
		display: flex;
		align-items: center;
		gap: 12px;
		min-width: 0;
	}
	.urv-file-path {
		font-family: var(--font-mono);
		font-size: var(--text-mono-sm);
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.urv-file-stats {
		display: inline-flex;
		gap: 6px;
		flex-shrink: 0;
	}
	.urv-stat-add {
		color: var(--success);
		font-family: var(--font-mono);
		font-size: var(--text-mono-2xs);
	}
	.urv-stat-del {
		color: var(--danger);
		font-family: var(--font-mono);
		font-size: var(--text-mono-2xs);
	}

	.urv-file-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-shrink: 0;
	}
	.urv-file-nav {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		border: 1px solid var(--border);
		border-radius: 6px;
		overflow: hidden;
	}
	.urv-nav-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 26px;
		padding: 0;
		border: none;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition: all var(--transition-fast);
	}
	.urv-nav-btn:hover:not(:disabled) {
		color: var(--text);
		background: var(--hover-bg);
	}
	.urv-nav-btn:disabled {
		opacity: 0.35;
		cursor: not-allowed;
	}
	.urv-nav-pos {
		font-size: var(--text-2xs);
		font-family: var(--font-mono);
		color: var(--muted);
		padding: 0 4px;
	}
	.urv-toggle-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		padding: 0;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition: all var(--transition-fast);
	}
	.urv-toggle-btn:hover {
		color: var(--text);
		background: var(--hover-bg);
	}
	.urv-toggle-btn.active {
		color: var(--accent);
		border-color: color-mix(in srgb, var(--accent) 40%, var(--border));
		background: color-mix(in srgb, var(--accent) 12%, transparent);
	}
	.urv-save-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		padding: 0;
		border: 1px solid color-mix(in srgb, var(--success) 40%, var(--border));
		border-radius: 6px;
		background: color-mix(in srgb, var(--success) 14%, transparent);
		color: var(--success);
		cursor: pointer;
		transition: all var(--transition-fast);
	}
	.urv-save-btn:hover:not(:disabled) {
		background: color-mix(in srgb, var(--success) 22%, transparent);
	}
	.urv-save-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.urv-editor {
		flex: 1;
		min-height: 0;
		min-width: 0;
		overflow: hidden;
		display: flex;
		flex-direction: column;
	}
	.urv-placeholder {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		color: var(--subtle);
		font-size: var(--text-base);
		text-align: center;
	}

	/* ── Markdown rendering ──────────────────────────────── */
	.urv-markdown-view {
		flex: 1;
		overflow-y: auto;
		padding: 20px 24px;
		color: var(--text);
		font-size: 13px;
		line-height: 1.68;
	}
	.urv-markdown-view :global(> *:first-child) {
		margin-top: 0;
	}
	.urv-markdown-view :global(h1),
	.urv-markdown-view :global(h2),
	.urv-markdown-view :global(h3),
	.urv-markdown-view :global(h4) {
		color: var(--text);
		margin: 1.2em 0 0.5em;
	}
	.urv-markdown-view :global(h1) {
		font-size: 22px;
		font-weight: 600;
		border-bottom: 1px solid var(--border);
		padding-bottom: 6px;
	}
	.urv-markdown-view :global(h2) {
		font-size: 18px;
		font-weight: 600;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
		padding-bottom: 4px;
	}
	.urv-markdown-view :global(h3) {
		font-size: 15px;
		font-weight: 600;
	}
	.urv-markdown-view :global(h4) {
		font-size: 14px;
		font-weight: 600;
	}
	.urv-markdown-view :global(p) {
		margin: 0.6em 0;
	}
	.urv-markdown-view :global(a) {
		color: var(--accent);
		text-decoration: none;
	}
	.urv-markdown-view :global(a:hover) {
		text-decoration: underline;
	}
	.urv-markdown-view :global(code) {
		font-family: var(--font-mono);
		font-size: 12px;
		background: var(--panel-strong);
		padding: 2px 5px;
		border-radius: 4px;
	}
	.urv-markdown-view :global(pre) {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 14px 16px;
		overflow-x: auto;
		margin: 0.8em 0;
	}
	.urv-markdown-view :global(pre code) {
		background: none;
		padding: 0;
		border-radius: 0;
	}
	.urv-markdown-view :global(ul),
	.urv-markdown-view :global(ol) {
		padding-left: 1.5em;
		margin: 0.5em 0;
	}
	.urv-markdown-view :global(li) {
		margin: 0.25em 0;
	}
	.urv-markdown-view :global(blockquote) {
		border-left: 3px solid var(--accent);
		padding-left: 14px;
		color: var(--muted);
		margin: 0.8em 0;
	}
	.urv-markdown-view :global(table) {
		border-collapse: collapse;
		width: 100%;
		margin: 0.8em 0;
	}
	.urv-markdown-view :global(th),
	.urv-markdown-view :global(td) {
		border: 1px solid var(--border);
		padding: 6px 12px;
		text-align: left;
	}
	.urv-markdown-view :global(th) {
		background: var(--panel-strong);
		font-weight: 600;
	}
	.urv-markdown-view :global(hr) {
		border: none;
		border-top: 1px solid var(--border);
		margin: 1.5em 0;
	}
	.urv-markdown-view :global(img) {
		max-width: 100%;
		border-radius: 6px;
	}
	/* Mermaid diagrams */
	.urv-markdown-view :global(.ws-mermaid-block) {
		margin: 1em 0;
	}
	.urv-markdown-view :global(.ws-mermaid-diagram) {
		display: flex;
		justify-content: center;
		padding: 16px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 8px;
		cursor: pointer;
	}
	.urv-markdown-view :global(.ws-mermaid-diagram svg) {
		max-width: 100%;
		height: auto;
	}
	.urv-markdown-view :global(.ws-mermaid-error) {
		font-size: 12px;
		color: var(--danger);
		padding: 8px;
	}
	/* Shiki code blocks inside markdown */
	.urv-markdown-view :global(.shiki) {
		background: var(--panel-strong) !important;
		border-radius: 8px;
		padding: 14px 16px;
		overflow-x: auto;
		font-size: 12px;
	}
	/* Badge images inline (shields.io etc.) */
	.urv-markdown-view :global(p > a > img[src*='shields.io']),
	.urv-markdown-view :global(p > a > img[src*='badge']),
	.urv-markdown-view :global(p > a > img[alt*='passing']),
	.urv-markdown-view :global(p > a > img[alt*='failing']) {
		display: inline;
		vertical-align: middle;
	}
	.urv-markdown-view :global(p > a:has(> img[src*='shields.io'])),
	.urv-markdown-view :global(p > a:has(> img[src*='badge'])) {
		display: inline;
		margin-right: 4px;
	}
	/* Make paragraphs that only contain badge links flow inline */
	.urv-markdown-view :global(p:has(> a > img[src*='shields.io']):not(:has(br ~ br))) {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		align-items: center;
	}

	/* ── Mermaid overlay ────────────────────────────────── */
	.mm-overlay {
		position: fixed;
		inset: 0;
		z-index: 100;
		background: rgba(0, 0, 0, 0.6);
		display: flex;
		align-items: center;
		justify-content: center;
		animation: sd-fade-in 120ms ease-out;
	}
	.mm-panel {
		width: 90vw;
		height: 85vh;
		max-width: 1200px;
		display: flex;
		flex-direction: column;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		box-shadow: var(--shadow-lg);
		overflow: hidden;
	}
	.mm-toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 16px;
		border-bottom: 1px solid var(--border);
		flex-shrink: 0;
	}
	.mm-zoom-actions {
		display: flex;
		align-items: center;
		gap: 4px;
	}
	.mm-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		padding: 0;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition: all var(--transition-fast);
	}
	.mm-btn:hover {
		color: var(--text);
		background: var(--hover-bg);
	}
	.mm-btn-text {
		padding: 4px 10px;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		cursor: pointer;
	}
	.mm-btn-text:hover {
		color: var(--text);
	}
	.mm-canvas {
		flex: 1;
		overflow: hidden;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.mm-surface {
		width: 100%;
		height: 100%;
		cursor: grab;
		display: flex;
		align-items: center;
		justify-content: center;
		overflow: hidden;
	}
	.mm-surface.dragging {
		cursor: grabbing;
	}
	.mm-stage {
		transform-origin: center center;
		transform: translate(var(--mm-x, 0), var(--mm-y, 0)) scale(var(--mm-scale, 1));
		width: var(--mm-w, auto);
		height: var(--mm-h, auto);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.mm-stage :global(svg) {
		max-width: 100%;
		max-height: 100%;
	}
</style>
