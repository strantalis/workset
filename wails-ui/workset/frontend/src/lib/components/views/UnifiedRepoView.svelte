<script lang="ts">
	import { untrack } from 'svelte';
	import {
		ChevronLeft,
		ChevronRight,
		Columns2,
		Edit3,
		Eye,
		FileCode,
		FolderTree,
		GitBranch,
		Minus,
		PanelLeftOpen,
		Plus,
		Rows2,
		BookOpen,
		Save,
		X,
	} from '@lucide/svelte';
	import type {
		RepoDiffFileSummary,
		RepoDiffSummary,
		RepoFileContent,
		RepoFileDefinitionTarget,
		RepoFileDiff,
		RepoFileSearchResult,
		Workspace,
		WorkspaceExtraRoot,
	} from '../../types';
	import {
		fetchRepoDiffSummary,
		fetchRepoFileDiff,
		fetchBranchDiffSummary,
		fetchBranchFileDiff,
	} from '../../api/repo-diff';
	import {
		fetchPullRequestReviews,
		fetchPullRequestStatus,
		fetchCheckAnnotations,
	} from '../../api/github/pull-request';
	import type { ReviewComment } from '../editor/reviewDecorations';
	import type { CIAnnotation } from '../editor/ciAnnotations';
	import {
		clearRepoFileSearchCache,
		readWorkspaceRepoFile,
		readWorkspaceRepoFileAtRef,
		searchWorkspaceRepoFiles,
		writeWorkspaceRepoFile,
		invalidateRepoFileContent,
		clearFileContentCache,
		listRepoDirectory,
		listWorkspaceExtraRoots,
		invalidateRepoDirCache,
		clearDirListCache,
		invalidateWorkspaceExtraRoots,
		clearWorkspaceExtraRootsCache,
		type RepoDirectoryEntry,
	} from '../../api/repo-files';
	import { resolveMarkdownImages, clearImageCache } from '../../markdownImages';
	import { buildSummaryLocalCacheKey, repoDiffCache } from '../../cache/repoDiffCache';
	import { subscribeRepoDiffEvent } from '../../repoDiffService';
	import { EVENT_REPO_DIFF_LOCAL_SUMMARY } from '../../events';
	import { applyTrackedPullRequest, refreshWorkspacesStatus } from '../../state';
	import {
		buildExpandedRepoTreeKeysForQuery,
		buildRepoTree,
		buildRepoTreeFromDirectories,
		computeRepoTreeChildCounts,
		computeRepoTreeDirectoryCounts,
		createRepoDirEntriesKey,
		shouldReplaceExpandedNodeSet,
		type RepoTreeNode,
	} from '../repo-files/tree';
	import { renderMarkdownDocument, type DocumentRenderResult } from '../../documentRender';
	import { calculateMermaidOverlayFit } from '../repo-files/mermaidOverlay';
	import ResizablePanel from '../ui/ResizablePanel.svelte';
	import CodeEditor from '../editor/CodeEditor.svelte';
	import CodeDiffView from '../editor/CodeDiffView.svelte';
	import PrCreateDrawer from './PrCreateDrawer.svelte';
	import LocalMergeDrawer from './LocalMergeDrawer.svelte';
	import PrLifecycleDrawer from './PrLifecycleDrawer.svelte';
	import UnifiedRepoSidebar from './UnifiedRepoSidebar.svelte';
	import {
		buildDirNodeKey,
		buildFileNodeKey,
		buildRepoNodeKey,
		type ExplorerTreeNode,
		getParentDirPath,
		insertInlineCreateNode,
		type InlineCreateState,
		removeDeletedDirectoryEntry,
		removeLoadedRepoFileState,
		resolveInlineCreate,
		type TreeSelection,
		upsertCreatedDirectoryEntries,
		upsertLoadedRepoFileState,
		validateInlineCreateFileName,
	} from './unifiedRepoInlineCreate';
	import {
		flushPendingRepoDefinitionTarget,
		navigateRepoDefinitionTarget,
	} from './unifiedRepoDefinition';
	import {
		findChangedFilePath,
		getRepoFileDiffInfo,
		handleEditorSaveKeydown,
		ignoreError,
		isRepoFileChanged,
		maybeLoadBranchDataForRepo,
	} from './unifiedRepoView.helpers';
	import { useNotifications } from '../../contexts/notifications';
	import { dirtyIndicator, setCleanDoc } from '../editor/dirtyIndicator';
	import { navigationKeymap } from '../editor/navigationKeymap';
	import { blameExtension, setBlameData } from '../editor/blameGutter';
	import {
		getRepoBlame,
		createWorkspaceRepoFile,
		deleteWorkspaceRepoFile,
	} from '../../api/repo-files';
	import type { EditorView } from '@codemirror/view';
	import type { Extension } from '@codemirror/state';
	import { buildReviewThreadCountsByFile } from '../../pullRequestUiHelpers';
	import { createRepoSemanticHoverExtensions } from './unifiedRepoHover';
	import './UnifiedRepoView.css';
	interface Props {
		workspace: Workspace | null;
		pendingFileSelection?: { repoId: string; path: string } | null;
		onFileSelectionHandled?: () => void;
	}
	const {
		workspace,
		pendingFileSelection = null,
		onFileSelectionHandled = () => {},
	}: Props = $props();
	const notifications = useNotifications();
	type RepoFileState =
		| { status: 'idle' }
		| { status: 'loading' }
		| { status: 'loaded'; files: RepoFileSearchResult[] }
		| { status: 'error'; message: string };
	let repoFileStates = $state<Map<string, RepoFileState>>(new Map());
	let extraRoots = $state<WorkspaceExtraRoot[]>([]);
	let dirEntries = $state<Map<string, RepoDirectoryEntry[]>>(new Map());
	let dirEntryErrors = $state<Map<string, string>>(new Map());
	let expandedNodes = $state<Set<string>>(new Set());
	let searchQuery = $state('');
	let showFileTree = $state(true);
	let repoDiffMap = $state<Map<string, RepoDiffSummary>>(new Map());
	let branchDiffMap = $state<Map<string, RepoDiffSummary>>(new Map());
	let prReviewComments = $state<ReviewComment[]>([]);
	let prCiAnnotations = $state<CIAnnotation[]>([]);
	let prFileCommentCounts = $state<Map<string, number>>(new Map());
	let blameMode = $state(false);
	let deleteConfirmPath = $state<string | null>(null);
	let deleteConfirmRepoId = $state<string | null>(null);
	let selectedRepoId: string | null = $state(null);
	let selectedFilePath: string | null = $state(null);
	let selectedTree = $state<TreeSelection>(null);
	let inlineCreate = $state<InlineCreateState | null>(null);
	let fileDiffContent: RepoFileDiff | null = $state(null);
	let fileDiffLoading = $state(false);
	let fileDiffError: string | null = $state(null);
	let fileDiffRequestId = 0;
	let originalFileContent: string | null = $state(null);
	let modifiedFileContent: string | null = $state(null);
	let fullDiffLoading = $state(false);
	let fullDiffRequestId = 0;
	let fileContent: RepoFileContent | null = $state(null);
	let fileContentLoading = $state(false);
	let fileContentRequestId = 0;
	let renderedMarkdown: DocumentRenderResult | null = $state(null);
	let renderLoading = $state(false);
	let renderToken = 0;
	let mermaidOverlayOpen = $state(false);
	let mermaidOverlayMarkup = $state('');
	let mermaidZoom = $state(1);
	let mermaidOffsetX = $state(0);
	let mermaidOffsetY = $state(0);
	let mermaidFitScale = $state(1);
	let mermaidDragging = $state(false);
	let mermaidDragPointerId = $state<number | null>(null);
	let mermaidDragOriginX = 0,
		mermaidDragOriginY = 0;
	let mermaidDragStartOffsetX = 0;
	let mermaidDragStartOffsetY = 0;
	let mermaidCanvasEl = $state<HTMLElement | null>(null);
	let mermaidStageEl = $state<HTMLElement | null>(null);
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
	const closeMermaid = (): void => void ((mermaidOverlayOpen = false), (mermaidOverlayMarkup = ''));
	const adjustMermaidZoom = (delta: number): void => {
		mermaidZoom = Math.min(2.5, Math.max(0.5, Math.round((mermaidZoom + delta) * 100) / 100));
	};
	// prettier-ignore
	const resetMermaidZoom = (): void => void ((mermaidZoom = 1), (mermaidOffsetX = 0), (mermaidOffsetY = 0));
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
	let unifiedDiff = $state(true);
	let editMode = $state(false);
	let previewMode = $state(false);
	let editedContent = $state<string | null>(null);
	let saving = $state(false);
	let editorView: EditorView | null = null;
	let editorViewPath = $state<string | null>(null);
	let editorViewVersion = $state(0);
	let pendingDefinitionTarget = $state<RepoFileDefinitionTarget | null>(null);

	// prettier-ignore
	const semanticHoverExtensions = $derived.by(() =>
		createRepoSemanticHoverExtensions(wsId, selectedRepoId, selectedFilePath, (target) => navigateRepoDefinitionTarget({ target, editorView, selectedRepoId, selectedFilePath, setPendingTarget: (nextTarget) => (pendingDefinitionTarget = nextTarget), selectTreeFile })),
	);
	const editExtensions = $derived.by((): Extension[] => [
		dirtyIndicator(),
		navigationKeymap({
			onPrevFile: () => navigateChangedFile(-1),
			onNextFile: () => navigateChangedFile(1),
		}),
		...semanticHoverExtensions,
	]);
	const viewExtensions = $derived.by((): Extension[] => [
		navigationKeymap({
			onPrevFile: () => navigateChangedFile(-1),
			onNextFile: () => navigateChangedFile(1),
		}),
		...semanticHoverExtensions,
		...(blameMode ? [blameExtension()] : []),
	]);
	// prettier-ignore
	const handleEditorReady = (view: EditorView): void => void ((editorView = view), (editorViewPath = selectedFilePath), (editorViewVersion += 1));
	const handleContentChange = (content: string): void => void (editedContent = content);
	const saveFile = async (): Promise<void> => {
		if (!editMode || saving || editedContent === null) return;
		if (!wsId || !selectedRepoId || !selectedFilePath) return;
		saving = true;
		try {
			const savedContent = editedContent;
			await writeWorkspaceRepoFile(wsId, selectedRepoId, selectedFilePath, editedContent);
			if (fileContent) {
				fileContent = { ...fileContent, content: editedContent };
			}
			editedContent = null;
			if (editorView) {
				editorView.dispatch({ effects: setCleanDoc.of(savedContent) });
			}
			notifications.info('File saved');
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Unknown error';
			notifications.error(`Save failed: ${msg}`);
		} finally {
			saving = false;
		}
	};
	const handleKeydown = (event: KeyboardEvent): void => {
		handleEditorSaveKeydown(event, editMode, () => void saveFile());
	};
	let focusedNodeIndex = $state(-1);
	const handleTreeKeydown = (event: KeyboardEvent): void => {
		const nodes = visibleTreeNodes;
		if (nodes.length === 0) return;

		let handled = true;
		switch (event.key) {
			case 'ArrowDown':
			case 'j':
				focusedNodeIndex = Math.min(focusedNodeIndex + 1, nodes.length - 1);
				break;
			case 'ArrowUp':
			case 'k':
				focusedNodeIndex = Math.max(focusedNodeIndex - 1, 0);
				break;
			case 'Enter':
			case ' ': {
				const node = nodes[focusedNodeIndex];
				if (!node) break;
				if (node.kind === 'inline-create') {
					break;
				}
				if (node.kind === 'file') {
					selectTreeFile(node.path, node.repoId);
				} else {
					toggleNode(node);
				}
				break;
			}
			case 'ArrowRight': {
				const node = nodes[focusedNodeIndex];
				if (
					node &&
					node.kind !== 'file' &&
					node.kind !== 'inline-create' &&
					!expandedNodes.has(node.key)
				) {
					toggleNode(node);
				}
				break;
			}
			case 'ArrowLeft': {
				const node = nodes[focusedNodeIndex];
				if (
					node &&
					node.kind !== 'file' &&
					node.kind !== 'inline-create' &&
					expandedNodes.has(node.key)
				) {
					toggleNode(node);
				}
				break;
			}
			case '[':
				navigateChangedFile(-1);
				break;
			case ']':
				navigateChangedFile(1);
				break;
			default:
				handled = false;
		}

		if (handled) {
			event.preventDefault();
			const treeList = event.currentTarget as HTMLElement;
			const focusedEl = treeList.children[focusedNodeIndex] as HTMLElement | undefined;
			focusedEl?.scrollIntoView({ block: 'nearest' });
		}
	};
	let drawerMode: 'none' | 'pr-create' | 'local-merge' | 'pr-lifecycle' = $state('none');
	const closeDrawer = (): void => void (drawerMode = 'none');
	const wsId = $derived(workspace?.id ?? '');
	const explorerRoots = $derived.by(() => {
		const repoRoots =
			workspace?.repos.map((repo) => ({
				id: repo.id,
				label: repo.name,
				kind: 'repo' as const,
				gitDetected: false,
			})) ?? [];
		const extraRootItems = extraRoots.map((root) => ({
			id: root.id,
			label: root.label,
			kind: 'extra' as const,
			gitDetected: root.gitDetected,
		}));
		return [...repoRoots, ...extraRootItems].sort((left, right) =>
			left.label.localeCompare(right.label),
		);
	});
	const explorerRootById = $derived.by(() => new Map(explorerRoots.map((root) => [root.id, root])));
	const repos = $derived.by(() => explorerRoots.map((root) => ({ id: root.id, name: root.label })));
	const searchableRepoIds = $derived.by(() =>
		explorerRoots.filter((root) => root.kind === 'repo').map((root) => root.id),
	);
	const selectedRepo = $derived.by(
		() => workspace?.repos.find((r) => r.id === selectedRepoId) ?? null,
	);

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
	const searchTargetRepoIds = $derived.by(() => {
		if (!isSearchActive) return [];
		const expandedRepoIds = [...expandedNodes]
			.filter((key) => key.startsWith('repo:'))
			.map((key) => key.slice(5));
		const searchableExpandedRepoIds = expandedRepoIds.filter((repoId) =>
			searchableRepoIds.includes(repoId),
		);
		if (!selectedRepoId || !searchableExpandedRepoIds.includes(selectedRepoId)) {
			return searchableExpandedRepoIds;
		}
		return [
			selectedRepoId,
			...searchableExpandedRepoIds.filter((repoId) => repoId !== selectedRepoId),
		];
	});
	const searchRepoLoadingCount = $derived.by(() => {
		let count = 0;
		for (const repoId of searchTargetRepoIds) {
			if (repoFileStates.get(repoId)?.status === 'loading') count += 1;
		}
		return count;
	});
	const treeNodes = $derived.by<RepoTreeNode[]>(() =>
		isSearchActive
			? buildRepoTree(
					repos.filter((root) => searchableRepoIds.includes(root.id)),
					filteredRepoFiles,
					expandedNodes,
				)
			: buildRepoTreeFromDirectories(repos, dirEntries, expandedNodes),
	);
	const visibleTreeNodes = $derived.by<ExplorerTreeNode[]>(() => {
		return insertInlineCreateNode(treeNodes, inlineCreate);
	});
	const pendingDeleteKey = $derived.by(() => {
		if (!deleteConfirmPath || !deleteConfirmRepoId) return null;
		return buildFileNodeKey(deleteConfirmRepoId, deleteConfirmPath);
	});
	const childCounts = $derived.by(() =>
		isSearchActive
			? computeRepoTreeChildCounts(filteredRepoFiles)
			: computeRepoTreeDirectoryCounts(dirEntries),
	);

	const activeDiffMap = $derived.by(() => {
		const merged = new Map(repoDiffMap);
		for (const [repoId, summary] of branchDiffMap) {
			const repo = workspace?.repos.find((r) => r.id === repoId);
			const pr = repo?.trackedPullRequest;
			if (pr && pr.state.toLowerCase() === 'open') {
				merged.set(repoId, summary);
			}
		}
		return merged;
	});

	const selectedRepoPr = $derived.by(() => {
		const repo = selectedRepo;
		const pr = repo?.trackedPullRequest;
		if (pr && pr.state.toLowerCase() === 'open') return pr;
		return null;
	});

	const changedFileSet = $derived.by(() => {
		const set = new Set<string>();
		for (const [repoId, summary] of activeDiffMap) {
			for (const file of summary.files) {
				set.add(`${repoId}:${file.path}`);
			}
		}
		return set;
	});

	const selectedDiffFile = $derived.by((): RepoDiffFileSummary | null => {
		if (!selectedRepoId || !selectedFilePath) return null;
		const summary = activeDiffMap.get(selectedRepoId);
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

	const currentRepoChangedFiles = $derived.by((): RepoDiffFileSummary[] => {
		if (!selectedRepoId) return [];
		return repoDiffMap.get(selectedRepoId)?.files ?? [];
	});
	const selectedChangedFileIdx = $derived.by(() => {
		if (!selectedFilePath || !selectedDiffFile) return -1;
		return currentRepoChangedFiles.findIndex((f) => f.path === selectedFilePath);
	});

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
	const dirCommentCount = $derived.by(() => {
		const map = new Map<string, number>();
		for (const [key, count] of prFileCommentCounts) {
			if (count <= 0) continue;
			const [repoId, path] = key.split('\u0000');
			if (!repoId || !path) continue;
			const parts = path.split('/');
			for (let i = 1; i < parts.length; i++) {
				const dirKey = `dir:${repoId}:${parts.slice(0, i).join('/')}`;
				map.set(dirKey, (map.get(dirKey) ?? 0) + count);
			}
		}
		return map;
	});

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
	const getOpenTrackedPrForRepo = (repoId: string) => {
		const repo = workspace?.repos.find((candidate) => candidate.id === repoId);
		const tracked = repo?.trackedPullRequest;
		if (tracked && tracked.state.toLowerCase() === 'open') return tracked;
		return null;
	};

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
		const key = createRepoDirEntriesKey(repoId, dirPath);
		if (dirEntries.has(key)) return; // already loaded
		void listRepoDirectory(workspaceId, repoId, dirPath)
			.then((entries) => {
				const nextErrors = new Map(dirEntryErrors);
				nextErrors.delete(key);
				dirEntryErrors = nextErrors;
				dirEntries = new Map(dirEntries).set(key, entries);
			})
			.catch((err) => {
				const message =
					err instanceof Error && err.message.trim().length > 0
						? err.message
						: 'Failed to load directory.';
				dirEntryErrors = new Map(dirEntryErrors).set(key, message);
			});
	};
	const loadWorkspaceExtraRootState = (workspaceId: string): void => {
		void listWorkspaceExtraRoots(workspaceId)
			.then((roots) => {
				extraRoots = roots;
			})
			.catch(() => {
				extraRoots = [];
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
			ignoreError();
		}
	};
	const loadBranchDiff = async (
		workspaceId: string,
		repoId: string,
		baseBranch: string,
		headBranch: string,
	): Promise<void> => {
		try {
			const fetched = await fetchBranchDiffSummary(workspaceId, repoId, baseBranch, headBranch);
			branchDiffMap = new Map(branchDiffMap).set(repoId, fetched);
		} catch {
			ignoreError();
		}
	};

	let allPrReviewCommentsByRepo = $state<
		Map<string, import('../../types').PullRequestReviewComment[]>
	>(new Map());
	const buildRepoFileCommentKey = (repoId: string, path: string): string =>
		`${repoId}\u0000${path}`;
	const replaceRepoFileCommentCounts = (
		current: Map<string, number>,
		repoId: string,
		nextCounts: Map<string, number>,
	): Map<string, number> => {
		const prefix = `${repoId}\u0000`;
		const merged = new Map<string, number>();
		for (const [key, value] of current) {
			if (!key.startsWith(prefix)) {
				merged.set(key, value);
			}
		}
		for (const [path, value] of nextCounts) {
			merged.set(buildRepoFileCommentKey(repoId, path), value);
		}
		return merged;
	};
	const getFileCommentCount = (repoId: string, path: string): number =>
		prFileCommentCounts.get(buildRepoFileCommentKey(repoId, path)) ?? 0;
	const loadAllPrReviewComments = async (wsId: string, repoId: string): Promise<void> => {
		const pr = getOpenTrackedPrForRepo(repoId);
		if (!pr) {
			const nextComments = new Map(allPrReviewCommentsByRepo);
			nextComments.delete(repoId);
			allPrReviewCommentsByRepo = nextComments;
			prFileCommentCounts = replaceRepoFileCommentCounts(prFileCommentCounts, repoId, new Map());
			return;
		}
		try {
			const comments = await fetchPullRequestReviews(wsId, repoId, pr.number, pr.headBranch);
			const nextComments = new Map(allPrReviewCommentsByRepo);
			nextComments.set(repoId, comments);
			allPrReviewCommentsByRepo = nextComments;

			const repoFiles =
				branchDiffMap.get(repoId)?.files ??
				repoDiffMap.get(repoId)?.files ??
				Array.from(
					new Set(comments.map((comment) => comment.path).filter((path) => path.trim().length > 0)),
				).map((path) => ({ path, added: 0, removed: 0, status: 'modified' }));
			const counts = buildReviewThreadCountsByFile(comments, repoFiles);
			prFileCommentCounts = replaceRepoFileCommentCounts(prFileCommentCounts, repoId, counts);
		} catch {
			const nextComments = new Map(allPrReviewCommentsByRepo);
			nextComments.delete(repoId);
			allPrReviewCommentsByRepo = nextComments;
			prFileCommentCounts = replaceRepoFileCommentCounts(prFileCommentCounts, repoId, new Map());
		}
	};
	const loadPrAnnotations = async (
		wsId: string,
		repoId: string,
		filePath: string,
	): Promise<void> => {
		const pr = selectedRepoPr;
		if (!pr) {
			prReviewComments = [];
			prCiAnnotations = [];
			return;
		}

		const repoComments = allPrReviewCommentsByRepo.get(repoId) ?? [];
		const fileComments = repoComments.filter((c) => c.path === filePath && c.line != null);
		prReviewComments = fileComments.map((c) => ({
			id: c.id,
			author: c.author ?? 'unknown',
			body: c.body,
			line: c.line!,
			path: c.path,
			createdAt: c.createdAt,
			resolved: c.resolved,
			threadId: c.threadId,
		}));

		try {
			const statusResult = await fetchPullRequestStatus(wsId, repoId, pr.number, pr.headBranch);
			const annotations: CIAnnotation[] = [];
			for (const check of statusResult.checks) {
				if (!check.checkRunId || check.conclusion === 'success') continue;
				try {
					const parts = pr.repo.split('/');
					if (parts.length !== 2) continue;
					const checkAnns = await fetchCheckAnnotations(parts[0], parts[1], check.checkRunId);
					for (const ann of checkAnns) {
						if (ann.path !== filePath) continue;
						annotations.push({
							line: ann.startLine,
							message: ann.message,
							severity:
								ann.level === 'failure' ? 'error' : ann.level === 'warning' ? 'warning' : 'notice',
							title: ann.title,
							path: ann.path,
						});
					}
				} catch {
					ignoreError();
				}
			}
			prCiAnnotations = annotations;
		} catch {
			prCiAnnotations = [];
		}
	};
	const loadBlame = async (wsId: string, repoId: string, path: string): Promise<void> => {
		try {
			const entries = await getRepoBlame(wsId, repoId, path);
			if (selectedRepoId === repoId && selectedFilePath === path && blameMode && editorView) {
				editorView.dispatch({ effects: setBlameData.of(entries) });
			}
		} catch {
			ignoreError();
		}
	};
	// prettier-ignore
	const selectRepoNode = (repoId: string): void => void ((selectedRepoId = repoId), (selectedTree = { kind: 'repo', key: buildRepoNodeKey(repoId), repoId }), (deleteConfirmPath = null), (deleteConfirmRepoId = null));
	// prettier-ignore
	const selectDirNode = (repoId: string, path: string): void => void ((selectedRepoId = repoId), (selectedTree = { kind: 'dir', key: buildDirNodeKey(repoId, path), repoId, path }), (deleteConfirmPath = null), (deleteConfirmRepoId = null));
	// prettier-ignore
	const cancelInlineCreate = (): void => void (inlineCreate = null);
	// prettier-ignore
	const setInlineCreateDraft = (draftName: string): void => void (inlineCreate && (inlineCreate = { ...inlineCreate, draftName }));
	const upsertCreatedFileInTree = (repoId: string, fullPath: string): void => {
		const parentDirPath = getParentDirPath(fullPath);
		const dirKey = createRepoDirEntriesKey(repoId, parentDirPath);
		dirEntries = upsertCreatedDirectoryEntries(
			dirEntries,
			dirKey,
			fullPath,
			isMarkdownPath(fullPath),
		);

		const rootLabel = explorerRootById.get(repoId)?.label ?? repoId;
		const existingState = repoFileStates.get(repoId);
		if (existingState?.status === 'loaded') {
			repoFileStates = new Map(repoFileStates).set(repoId, {
				status: 'loaded',
				files: upsertLoadedRepoFileState(
					existingState.files,
					wsId,
					repoId,
					rootLabel,
					fullPath,
					isMarkdownPath(fullPath),
				),
			});
		}
	};
	const startInlineCreate = (): void => {
		const fallbackRepoId = selectedRepoId ?? explorerRoots[0]?.id ?? null;
		if (!fallbackRepoId) {
			notifications.error('No repository or folder is available for new files.');
			return;
		}

		const resolution = resolveInlineCreate(selectedTree, fallbackRepoId, expandedNodes);
		expandedNodes = resolution.nextExpandedNodes;
		if (wsId) {
			for (const dirPath of resolution.dirPathsToLoad) {
				loadDirEntries(wsId, resolution.inlineCreate.repoId, dirPath);
			}
		}
		if (resolution.shouldSelectRepo) {
			selectRepoNode(resolution.inlineCreate.repoId);
		}
		inlineCreate = resolution.inlineCreate;
	};
	const handleCreateFile = async (): Promise<void> => {
		const createState = inlineCreate;
		if (!createState || !wsId) return;
		const fileName = createState.draftName.trim();
		const validationError = validateInlineCreateFileName(fileName);
		if (validationError) {
			notifications.error(validationError);
			return;
		}

		const path =
			createState.parentDirPath === '' ? fileName : `${createState.parentDirPath}/${fileName}`;
		inlineCreate = { ...createState, creating: true };
		try {
			await createWorkspaceRepoFile(wsId, createState.repoId, path);
			notifications.info(`Created ${path}`);
			clearRepoFileSearchCache();
			invalidateRepoDirCache(wsId, createState.repoId);
			invalidateRepoFileContent(wsId, createState.repoId);
			upsertCreatedFileInTree(createState.repoId, path);
			inlineCreate = null;
			selectTreeFile(path, createState.repoId);
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Unknown error';
			inlineCreate = { ...createState, creating: false };
			notifications.error(`Create failed: ${msg}`);
		}
	};
	const undoDeletedFile = async (
		workspaceId: string,
		repoId: string,
		path: string,
		content: string,
	): Promise<void> => {
		try {
			await createWorkspaceRepoFile(workspaceId, repoId, path, content);
			clearRepoFileSearchCache();
			invalidateRepoDirCache(workspaceId, repoId);
			invalidateRepoFileContent(workspaceId, repoId);
			upsertCreatedFileInTree(repoId, path);
			selectTreeFile(path, repoId);
			notifications.info(`Restored ${path}`);
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Unknown error';
			notifications.error(`Undo delete failed: ${msg}`);
		}
	};
	const handleDeleteFile = async (): Promise<void> => {
		if (!deleteConfirmPath || !deleteConfirmRepoId || !wsId) return;
		const repoId = deleteConfirmRepoId;
		const path = deleteConfirmPath;
		let undoContent: string | null = null;
		try {
			const existingFile = await readWorkspaceRepoFile(wsId, repoId, path);
			if (!existingFile.isBinary && !existingFile.isTruncated) {
				undoContent = existingFile.content;
			}
		} catch {
			undoContent = null;
		}
		try {
			await deleteWorkspaceRepoFile(wsId, repoId, path);
			invalidateRepoDirCache(wsId, repoId);
			invalidateRepoFileContent(wsId, repoId);
			const parentDirPath = getParentDirPath(path);
			dirEntries = removeDeletedDirectoryEntry(
				dirEntries,
				createRepoDirEntriesKey(repoId, parentDirPath),
				path,
			);
			const existingState = repoFileStates.get(repoId);
			if (existingState?.status === 'loaded') {
				repoFileStates = new Map(repoFileStates).set(repoId, {
					status: 'loaded',
					files: removeLoadedRepoFileState(existingState.files, path),
				});
			}
			if (selectedRepoId === repoId && selectedFilePath === path) {
				editorView = null;
				editorViewPath = null;
				editorViewVersion += 1;
				selectedFilePath = null;
				fileContent = null;
				selectedTree =
					parentDirPath === ''
						? { kind: 'repo', key: buildRepoNodeKey(repoId), repoId }
						: {
								kind: 'dir',
								key: buildDirNodeKey(repoId, parentDirPath),
								repoId,
								path: parentDirPath,
							};
			}
			if (undoContent !== null) {
				const workspaceId = wsId;
				notifications.info(`Deleted ${path}`, {
					duration: 10_000,
					actionLabel: 'Undo',
					onAction: () => undoDeletedFile(workspaceId, repoId, path, undoContent ?? ''),
				});
			} else {
				notifications.info(`Deleted ${path}`);
			}
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Unknown error';
			notifications.error(`Delete failed: ${msg}`);
		} finally {
			deleteConfirmPath = null;
			deleteConfirmRepoId = null;
		}
	};

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
	const refreshExplorerTree = (): void => {
		const currentWsId = wsId;
		if (!currentWsId) return;

		inlineCreate = null;
		clearRepoFileSearchCache();
		invalidateWorkspaceExtraRoots(currentWsId);
		loadWorkspaceExtraRootState(currentWsId);
		clearDirListCache();
		dirEntries = new Map();
		dirEntryErrors = new Map();

		const expandedRepoIds = [...expandedNodes]
			.filter((key) => key.startsWith('repo:'))
			.map((key) => key.slice(5));
		for (const repoId of expandedRepoIds) {
			loadDirEntries(currentWsId, repoId, '');
			if (explorerRootById.get(repoId)?.kind === 'repo') {
				void loadRepoDiff(currentWsId, repoId, true);
				/* prettier-ignore */ maybeLoadBranchDataForRepo(workspace, currentWsId, repoId, loadBranchDiff, loadAllPrReviewComments);
			}
		}
		for (const node of treeNodes) {
			if (node.kind !== 'dir' || !expandedNodes.has(node.key)) continue;
			loadDirEntries(currentWsId, node.repoId, node.path);
		}
		if (selectedRepoId) {
			invalidateRepoFileContent(currentWsId, selectedRepoId);
			refreshCurrentFile();
		}
	};
	const getDirEntryError = (repoId: string, dirPath: string): string | undefined =>
		dirEntryErrors.get(createRepoDirEntriesKey(repoId, dirPath));
	const getRootState = (
		repoId: string,
	): { status: 'idle' | 'loading' | 'loaded' | 'error'; message?: string } | undefined => {
		const state = repoFileStates.get(repoId);
		if (!state) return undefined;
		if (state.status === 'error') {
			return { status: state.status, message: state.message };
		}
		return { status: state.status };
	};
	const toggleNode = (node: Extract<RepoTreeNode, { kind: 'repo' | 'dir' }>): void => {
		inlineCreate = null;
		deleteConfirmPath = null;
		deleteConfirmRepoId = null;
		if (node.kind === 'repo') {
			selectRepoNode(node.repoId);
		} else {
			selectDirNode(node.repoId, node.path);
		}
		const { key } = node;
		const next = new Set(expandedNodes);
		if (next.has(key)) {
			next.delete(key);
			if (node.kind === 'repo' && explorerRootById.get(node.repoId)?.kind === 'repo') {
				const { repoId } = node;
				if (repoDiffMap.has(repoId)) {
					const nextMap = new Map(repoDiffMap);
					nextMap.delete(repoId);
					repoDiffMap = nextMap;
				}
				if (branchDiffMap.has(repoId)) {
					const nextMap = new Map(branchDiffMap);
					nextMap.delete(repoId);
					branchDiffMap = nextMap;
				}
			}
		} else {
			next.add(key);
			if (node.kind === 'repo' && wsId) {
				const { repoId } = node;
				loadDirEntries(wsId, repoId, '');
				if (explorerRootById.get(repoId)?.kind === 'repo') {
					void loadRepoDiff(wsId, repoId);
					/* prettier-ignore */ maybeLoadBranchDataForRepo(workspace, wsId, repoId, loadBranchDiff, loadAllPrReviewComments);
				}
			} else if (node.kind === 'dir' && wsId) {
				loadDirEntries(wsId, node.repoId, node.path);
			}
		}
		expandedNodes = next;
	};
	const selectTreeFile = (path: string, repoId: string): void => {
		const sameFile = selectedRepoId === repoId && selectedFilePath === path;
		if (!sameFile) {
			editorView = null;
			editorViewPath = null;
			editorViewVersion += 1;
		}
		selectedRepoId = repoId;
		selectedFilePath = path;
		selectedTree = { kind: 'file', key: buildFileNodeKey(repoId, path), repoId, path };
		inlineCreate = null;
		deleteConfirmPath = null;
		deleteConfirmRepoId = null;
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

	$effect(() => {
		if (pendingFileSelection) {
			const { repoId, path } = pendingFileSelection;
			const repoKey = `repo:${repoId}`;
			if (!expandedNodes.has(repoKey)) {
				expandedNodes = new Set([...expandedNodes, repoKey]);
			}
			selectTreeFile(path, repoId);
			onFileSelectionHandled();
		}
	});
	// prettier-ignore
	const navigateChangedFile = (delta: number): void => { const nextPath = findChangedFilePath(currentRepoChangedFiles, selectedChangedFileIdx, delta); if (nextPath && selectedRepoId) selectTreeFile(nextPath, selectedRepoId); };
	const loadFileDiff = async (
		wsId: string,
		repoId: string,
		file: RepoDiffFileSummary,
	): Promise<void> => {
		const requestId = ++fileDiffRequestId;
		fileDiffLoading = true;
		fileDiffError = null;
		try {
			let fetched;
			const pr = selectedRepoPr;
			if (pr) {
				fetched = await fetchBranchFileDiff(
					wsId,
					repoId,
					pr.baseBranch,
					pr.headBranch,
					file.path,
					file.prevPath ?? '',
				);
			} else {
				fetched = await fetchRepoFileDiff(
					wsId,
					repoId,
					file.path,
					file.prevPath ?? '',
					file.status ?? '',
				);
			}
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
			const pr = selectedRepoPr;
			let origResult, modResult;
			if (pr) {
				[origResult, modResult] = await Promise.all([
					readWorkspaceRepoFileAtRef(wsId, repoId, path, pr.baseBranch),
					readWorkspaceRepoFileAtRef(wsId, repoId, path, pr.headBranch),
				]);
				if (requestId !== fullDiffRequestId) return;
				originalFileContent = origResult.found ? origResult.content : '';
				modifiedFileContent = modResult.found ? modResult.content : '';
			} else {
				[origResult, modResult] = await Promise.all([
					readWorkspaceRepoFileAtRef(wsId, repoId, path, 'HEAD'),
					readWorkspaceRepoFile(wsId, repoId, path),
				]);
				if (requestId !== fullDiffRequestId) return;
				originalFileContent = origResult.found ? origResult.content : '';
				modifiedFileContent = modResult.content;
			}
		} catch {
			if (requestId !== fullDiffRequestId) return;
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

	$effect(() => {
		const currentWsId = wsId;
		if (!currentWsId) return;
		if (!isSearchActive) return;
		if (searchRepoLoadingCount > 0) return;
		const nextRepoId = searchTargetRepoIds.find((repoId) => {
			const state = repoFileStates.get(repoId);
			return !state || state.status === 'idle' || state.status === 'error';
		});
		if (nextRepoId) {
			loadRepoFiles(currentWsId, nextRepoId);
		}
	});

	$effect(() => {
		const query = searchQuery.trim().toLowerCase();
		if (query.length === 0) return;
		const toExpand = buildExpandedRepoTreeKeysForQuery(filteredRepoFiles);
		const next = new Set(expandedNodes);
		for (const key of toExpand) next.add(key);
		if (shouldReplaceExpandedNodeSet(expandedNodes, next)) {
			expandedNodes = next;
		}
	});

	let lastWorkspaceId = '';
	$effect(() => {
		const ws = workspace;
		if (!ws) return;
		if (ws.id === lastWorkspaceId) return;
		lastWorkspaceId = ws.id;
		repoFileStates = new Map();
		extraRoots = [];
		dirEntries = new Map();
		dirEntryErrors = new Map();
		repoDiffMap = new Map();
		branchDiffMap = new Map();
		prFileCommentCounts = new Map();
		allPrReviewCommentsByRepo = new Map();
		selectedRepoId = null;
		editorView = null;
		editorViewPath = null;
		editorViewVersion += 1;
		selectedFilePath = null;
		selectedTree = null;
		inlineCreate = null;
		fileDiffContent = null;
		fileContent = null;
		expandedNodes = new Set();
		searchQuery = '';
		clearRepoFileSearchCache();
		clearImageCache();
		clearFileContentCache();
		clearDirListCache();
		clearWorkspaceExtraRootsCache();
		pendingDefinitionTarget = null;
		loadWorkspaceExtraRootState(ws.id);
		const firstRepo = ws.repos[0];
		if (firstRepo) {
			expandedNodes = new Set([`repo:${firstRepo.id}`]);
			selectRepoNode(firstRepo.id);
			untrack(() => {
				loadDirEntries(ws.id, firstRepo.id, '');
				void loadRepoDiff(ws.id, firstRepo.id);
				/* prettier-ignore */ maybeLoadBranchDataForRepo(workspace, ws.id, firstRepo.id, loadBranchDiff, loadAllPrReviewComments);
			});
		}
	});

	$effect(() => {
		const currentWsId = wsId;
		if (!currentWsId) return;
		if (workspace?.repos.length) return;
		if (expandedNodes.size > 0) return;
		const firstExtraRoot = extraRoots[0];
		if (!firstExtraRoot) return;

		expandedNodes = new Set([`repo:${firstExtraRoot.id}`]);
		selectRepoNode(firstExtraRoot.id);
		untrack(() => {
			loadDirEntries(currentWsId, firstExtraRoot.id, '');
		});
	});

	$effect(() => {
		const currentWsId = wsId;
		const repoId = selectedRepoId;
		const path = selectedFilePath;
		const diffFile = selectedDiffFile;
		const editing = editMode;
		if (!currentWsId || !repoId || !path) return;
		if (editing || previewMode) {
			if (!fileContent || fileContent.path !== path) {
				void loadFileContent(currentWsId, repoId, path);
			}
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

	$effect(() => {
		const currentWsId = wsId;
		const repoId = selectedRepoId;
		const pr = selectedRepoPr;
		const drawerOpen = drawerMode === 'pr-lifecycle';
		if (!drawerOpen || !currentWsId || !repoId || !pr) return;

		void loadAllPrReviewComments(currentWsId, repoId);
		const timer = setInterval(() => {
			void loadAllPrReviewComments(currentWsId, repoId);
		}, 10_000);
		return () => clearInterval(timer);
	});

	$effect(() => {
		const currentWsId = wsId;
		const repoId = selectedRepoId;
		const path = selectedFilePath;
		const pr = selectedRepoPr;
		if (!currentWsId || !repoId || !path || !pr) {
			prReviewComments = [];
			prCiAnnotations = [];
			return;
		}
		void loadPrAnnotations(currentWsId, repoId, path);
	});

	$effect(() => {
		const blame = blameMode;
		const currentWsId = wsId;
		const repoId = selectedRepoId;
		const path = selectedFilePath;
		if (
			!blame ||
			!currentWsId ||
			!repoId ||
			!path ||
			editMode ||
			explorerRootById.get(repoId)?.kind !== 'repo'
		) {
			return;
		}
		void loadBlame(currentWsId, repoId, path);
	});

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
						ignoreError();
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

	$effect(() => {
		if (mermaidOverlayOpen && mermaidOverlayMarkup) {
			queueMicrotask(updateMermaidFit);
		}
	});

	$effect(() => {
		const currentWsId = wsId;
		if (!currentWsId) return;
		const unsub = subscribeRepoDiffEvent<{
			workspaceId: string;
			repoId: string;
			summary: import('../../types').RepoDiffSummary;
		}>(EVENT_REPO_DIFF_LOCAL_SUMMARY, (payload) => {
			if (payload.workspaceId !== currentWsId) return;
			repoDiffMap = new Map(repoDiffMap).set(payload.repoId, payload.summary);
			repoDiffCache.setSummary(
				buildSummaryLocalCacheKey(currentWsId, payload.repoId),
				payload.summary,
			);
			invalidateRepoFileContent(currentWsId, payload.repoId);
			invalidateRepoDirCache(currentWsId, payload.repoId);
			if (payload.repoId === selectedRepoId && selectedFilePath) {
				refreshCurrentFile();
			}
		});
		return unsub;
	});

	$effect(() => {
		const target = pendingDefinitionTarget;
		const view = editorView;
		const viewPath = editorViewPath;
		const viewVersion = editorViewVersion;
		/* prettier-ignore */ flushPendingRepoDefinitionTarget({ target, editorView: view, editorViewPath: viewPath, selectedRepoId, selectedFilePath, isCurrent: () => pendingDefinitionTarget === target && editorView === view && editorViewPath === viewPath && editorViewVersion === viewVersion, setPendingTarget: (nextTarget) => (pendingDefinitionTarget = nextTarget) });
	});

	// prettier-ignore
	const isFileChanged = (repoId: string, path: string): boolean => isRepoFileChanged(changedFileSet, repoId, path);
	// prettier-ignore
	const getFileDiffInfo = (repoId: string, path: string): RepoDiffFileSummary | undefined => getRepoFileDiffInfo(repoDiffMap, repoId, path);
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="urv">
	{#if !workspace}
		<div class="urv-empty">
			<FolderTree size={48} />
			<p>Select a thread to view files</p>
		</div>
	{:else if explorerRoots.length === 0}
		<div class="urv-empty">
			<FolderTree size={48} />
			<p>No repositories or workspace folders in this workspace</p>
		</div>
	{:else}
		{#snippet sidebarPane()}
			<UnifiedRepoSidebar
				showRepoActions={selectedRepo !== null}
				showSelectedPrAction={selectedRepo?.trackedPullRequest?.state?.toLowerCase() === 'open'}
				{searchQuery}
				treeNodes={visibleTreeNodes}
				{focusedNodeIndex}
				{expandedNodes}
				{childCounts}
				{repoChangeStats}
				{changedDirSet}
				{dirChangeCount}
				{dirCommentCount}
				selectedTreeKey={selectedTree?.key ?? null}
				{pendingDeleteKey}
				{selectedRepoId}
				{selectedFilePath}
				{editedContent}
				inlineCreateDraft={inlineCreate?.draftName ?? ''}
				inlineCreatePending={inlineCreate?.creating ?? false}
				getRootMeta={(repoId) => explorerRootById.get(repoId)}
				{getRepoPrState}
				{getRootState}
				{getDirEntryError}
				{isFileChanged}
				{getFileDiffInfo}
				{getFileCommentCount}
				onOpenSelectedPr={() => (drawerMode = 'pr-lifecycle')}
				onCreatePr={() => (drawerMode = 'pr-create')}
				onLocalMerge={() => (drawerMode = 'local-merge')}
				onRefresh={refreshExplorerTree}
				onNewFile={startInlineCreate}
				onHideTree={() => (showFileTree = false)}
				onSearchQueryChange={(value) => (searchQuery = value)}
				onToggleNode={toggleNode}
				onTreeKeydown={handleTreeKeydown}
				onInlineCreateDraftChange={setInlineCreateDraft}
				onCommitInlineCreate={() => void handleCreateFile()}
				onCancelInlineCreate={cancelInlineCreate}
				onOpenTrackedPr={(repoId) => {
					selectRepoNode(repoId);
					drawerMode = 'pr-lifecycle';
				}}
				onSelectFile={(repoId, path) => selectTreeFile(path, repoId)}
				onDeleteFile={(repoId, path) => {
					deleteConfirmRepoId = repoId;
					deleteConfirmPath = path;
				}}
				onConfirmDelete={() => void handleDeleteFile()}
				onCancelDelete={() => {
					deleteConfirmRepoId = null;
					deleteConfirmPath = null;
				}}
			/>
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
							{#if !editMode && !isChangedFile && selectedRepo}
								<button
									type="button"
									class="urv-toggle-btn"
									class:active={blameMode}
									aria-label={blameMode ? 'Hide blame' : 'Show blame'}
									title={blameMode ? 'Hide blame' : 'Show blame'}
									onclick={() => (blameMode = !blameMode)}
								>
									<GitBranch size={13} />
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
									if (editMode) blameMode = false;
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
								extensions={editExtensions}
								onContentChange={handleContentChange}
								onViewReady={handleEditorReady}
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
								reviewComments={prReviewComments}
								ciAnnotations={prCiAnnotations}
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
								extensions={viewExtensions}
								onViewReady={handleEditorReady}
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
		onclick={closeMermaid}
		onkeydown={(e) => {
			if (e.key === 'Escape') closeMermaid();
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
				<button type="button" class="mm-btn" aria-label="Close" onclick={closeMermaid}
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
		onCreated={(created) => {
			applyTrackedPullRequest(workspace.id, selectedRepo.id, created);
			closeDrawer();
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
	{@const prDiffSummary = branchDiffMap.get(selectedRepo.id)}
	{@const repoCommentCounts = prFileCommentCounts}
	<PrLifecycleDrawer
		open={drawerMode === 'pr-lifecycle'}
		workspaceId={workspace.id}
		repoId={selectedRepo.id}
		repoName={selectedRepo.name}
		branch={selectedRepo.currentBranch || 'main'}
		trackedPr={selectedRepo.trackedPullRequest ?? null}
		diffStats={prDiffSummary
			? {
					filesChanged: prDiffSummary.files.length,
					additions: prDiffSummary.totalAdded,
					deletions: prDiffSummary.totalRemoved,
				}
			: null}
		unresolvedThreads={(() => {
			let total = 0;
			for (const count of repoCommentCounts.values()) total += count;
			return total;
		})()}
		onClose={closeDrawer}
		onStatusChanged={() => {}}
	/>
{/if}
