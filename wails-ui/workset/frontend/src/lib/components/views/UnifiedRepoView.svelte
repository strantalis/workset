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
		FilePlus,
		FolderTree,
		GitBranch,
		GitMerge,
		GitPullRequest,
		LoaderCircle,
		MessageCircle,
		Minus,
		PanelLeftClose,
		PanelLeftOpen,
		Plus,
		Rows2,
		BookOpen,
		Save,
		Search,
		Trash2,
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
		fetchBranchDiffSummary,
		fetchBranchFileDiff,
		startRepoStatusWatch,
		stopRepoStatusWatch,
	} from '../../api/repo-diff';
	import {
		fetchPullRequestReviews,
		fetchPullRequestStatus,
		fetchCheckAnnotations,
	} from '../../api/github/pull-request';
	import type { ReviewComment } from '../editor/reviewDecorations';
	import type { CIAnnotation } from '../editor/ciAnnotations';
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
	import { getRepoFileIcon } from '../repo-files/fileIcons';
	import { renderMarkdownDocument, type DocumentRenderResult } from '../../documentRender';
	import { calculateMermaidOverlayFit } from '../repo-files/mermaidOverlay';
	import ResizablePanel from '../ui/ResizablePanel.svelte';
	import CodeEditor from '../editor/CodeEditor.svelte';
	import CodeDiffView from '../editor/CodeDiffView.svelte';
	import PrCreateDrawer from './PrCreateDrawer.svelte';
	import LocalMergeDrawer from './LocalMergeDrawer.svelte';
	import PrLifecycleDrawer from './PrLifecycleDrawer.svelte';
	import { useNotifications } from '../../contexts/notifications';
	import { dirtyIndicator, setCleanDoc } from '../editor/dirtyIndicator';
	import { navigationKeymap } from '../editor/navigationKeymap';
	import { blameExtension, setBlameData } from '../editor/blameGutter';
	import {
		getRepoBlame,
		createWorkspaceRepoFile,
		deleteWorkspaceRepoFile,
	} from '../../api/repo-files';
	import Modal from '../Modal.svelte';
	import type { EditorView } from '@codemirror/view';
	import type { Extension } from '@codemirror/state';
	import { buildReviewThreadCountsByFile } from '../../pullRequestUiHelpers';
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
	let dirEntries = $state<Map<string, RepoDirectoryEntry[]>>(new Map());
	let dirEntryErrors = $state<Map<string, string>>(new Map());
	let expandedNodes = $state<Set<string>>(new Set());
	let searchQuery = $state('');
	let showFileTree = $state(true);
	let repoDiffMap = $state<Map<string, RepoDiffSummary>>(new Map());
	let branchDiffMap = $state<Map<string, RepoDiffSummary>>(new Map());
	let prReviewComments = $state<ReviewComment[]>([]);
	let prCiAnnotations = $state<CIAnnotation[]>([]);
	// Per-file unresolved review thread counts for tree badges (repoId:path → count)
	let prFileCommentCounts = $state<Map<string, number>>(new Map());
	let blameMode = $state(false);
	let newFileDialogOpen = $state(false);
	let newFilePath = $state('');
	let deleteConfirmPath = $state<string | null>(null);
	let deleteConfirmRepoId = $state<string | null>(null);
	let selectedRepoId: string | null = $state(null);
	let selectedFilePath: string | null = $state(null);
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
	let mermaidDragOriginX = 0;
	let mermaidDragOriginY = 0;
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
	let unifiedDiff = $state(true);
	let editMode = $state(false);
	let previewMode = $state(false);
	let editedContent = $state<string | null>(null);
	let saving = $state(false);
	let editorView: EditorView | null = null;

	// CM6 extensions for edit mode: dirty indicator + file navigation keymap
	const editExtensions: Extension[] = [
		dirtyIndicator(),
		navigationKeymap({
			onPrevFile: () => navigateChangedFile(-1),
			onNextFile: () => navigateChangedFile(1),
		}),
	];
	// CM6 extensions for read-only mode: file navigation keymap + optional blame
	const viewExtensions = $derived.by((): Extension[] => {
		const exts: Extension[] = [
			navigationKeymap({
				onPrevFile: () => navigateChangedFile(-1),
				onNextFile: () => navigateChangedFile(1),
			}),
		];
		if (blameMode) exts.push(blameExtension());
		return exts;
	});
	const handleEditorReady = (view: EditorView): void => {
		editorView = view;
	};
	const handleContentChange = (content: string): void => {
		editedContent = content;
	};
	const saveFile = async (): Promise<void> => {
		if (!editMode || saving || editedContent === null) return;
		if (!wsId || !selectedRepoId || !selectedFilePath) return;
		saving = true;
		try {
			const savedContent = editedContent;
			await writeWorkspaceRepoFile(wsId, selectedRepoId, selectedFilePath, editedContent);
			// Update local state to reflect saved content
			if (fileContent) {
				fileContent = { ...fileContent, content: editedContent };
			}
			editedContent = null;
			// Tell CM6 dirty indicator the doc is now clean
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
		if ((event.metaKey || event.ctrlKey) && event.key === 's' && editMode) {
			event.preventDefault();
			void saveFile();
		}
	};
	let focusedNodeIndex = $state(-1);
	const handleTreeKeydown = (event: KeyboardEvent): void => {
		const nodes = treeNodes;
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
				if (node.kind === 'file') {
					selectTreeFile(node.path, node.repoId);
				} else {
					toggleNode(node);
				}
				break;
			}
			case 'ArrowRight': {
				const node = nodes[focusedNodeIndex];
				if (node && node.kind !== 'file' && !expandedNodes.has(node.key)) {
					toggleNode(node);
				}
				break;
			}
			case 'ArrowLeft': {
				const node = nodes[focusedNodeIndex];
				if (node && node.kind !== 'file' && expandedNodes.has(node.key)) {
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
			// Scroll focused node into view
			const treeList = event.currentTarget as HTMLElement;
			const focusedEl = treeList.children[focusedNodeIndex] as HTMLElement | undefined;
			focusedEl?.scrollIntoView({ block: 'nearest' });
		}
	};
	let drawerMode: 'none' | 'pr-create' | 'local-merge' | 'pr-lifecycle' = $state('none');
	const closeDrawer = (): void => void (drawerMode = 'none');
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
	const treeNodes = $derived.by<RepoTreeNode[]>(() =>
		isSearchActive
			? buildRepoTree(repos, filteredRepoFiles, expandedNodes)
			: buildRepoTreeFromDirectories(repos, dirEntries, expandedNodes),
	);
	const childCounts = $derived.by(() =>
		isSearchActive
			? computeRepoTreeChildCounts(filteredRepoFiles)
			: computeRepoTreeDirectoryCounts(dirEntries),
	);

	// Active diff map based on mode
	// When a repo has a tracked PR, prefer branch diffs; otherwise use working-tree diffs.
	// Merge both: branch diffs for repos with PRs, working-tree diffs for the rest.
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

	// Determine if repo has an open tracked PR (for branch diff mode)
	const selectedRepoPr = $derived.by(() => {
		const repo = selectedRepo;
		const pr = repo?.trackedPullRequest;
		if (pr && pr.state.toLowerCase() === 'open') return pr;
		return null;
	});

	// Changed files set: "repoId:path" for quick lookup
	const changedFileSet = $derived.by(() => {
		const set = new Set<string>();
		for (const [repoId, summary] of activeDiffMap) {
			for (const file of summary.files) {
				set.add(`${repoId}:${file.path}`);
			}
		}
		return set;
	});

	// Get diff info for the currently selected file
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
			// keep existing if available
		}
	};

	// All PR review comments keyed by repo, used for tree badges + per-file filtering
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

		// Filter cached comments for this file
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

		// Fetch CI annotations
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
					// skip this check run
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
			// blame failed — ignore silently
		}
	};
	const handleCreateFile = async (): Promise<void> => {
		const path = newFilePath.trim();
		if (!path || !wsId || !selectedRepoId) return;
		try {
			await createWorkspaceRepoFile(wsId, selectedRepoId, path);
			notifications.info(`Created ${path}`);
			invalidateRepoDirCache(wsId, selectedRepoId);
			invalidateRepoFileContent(wsId, selectedRepoId);
			newFileDialogOpen = false;
			newFilePath = '';
			selectTreeFile(path, selectedRepoId);
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Unknown error';
			notifications.error(`Create failed: ${msg}`);
		}
	};
	const handleDeleteFile = async (): Promise<void> => {
		if (!deleteConfirmPath || !deleteConfirmRepoId || !wsId) return;
		const repoId = deleteConfirmRepoId;
		const path = deleteConfirmPath;
		try {
			await deleteWorkspaceRepoFile(wsId, repoId, path);
			notifications.info(`Deleted ${path}`);
			invalidateRepoDirCache(wsId, repoId);
			invalidateRepoFileContent(wsId, repoId);
			if (selectedRepoId === repoId && selectedFilePath === path) {
				selectedFilePath = null;
				fileContent = null;
			}
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Unknown error';
			notifications.error(`Delete failed: ${msg}`);
		} finally {
			deleteConfirmPath = null;
			deleteConfirmRepoId = null;
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
	const getDirEntryError = (repoId: string, dirPath: string): string | undefined =>
		dirEntryErrors.get(createRepoDirEntriesKey(repoId, dirPath));
	const toggleNode = (node: Extract<RepoTreeNode, { kind: 'repo' | 'dir' }>): void => {
		const { key } = node;
		const next = new Set(expandedNodes);
		if (next.has(key)) {
			next.delete(key);
			// Evict diff data and stop watcher for collapsed repos
			if (node.kind === 'repo') {
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
				stopWatcherForRepo(repoId);
			}
		} else {
			next.add(key);
			if (node.kind === 'repo' && wsId) {
				const { repoId } = node;
				selectedRepoId = repoId;
				// Lazy-load root directory entries for browsing
				loadDirEntries(wsId, repoId, '');
				void loadRepoDiff(wsId, repoId);
				maybeLoadBranchData(wsId, repoId);
				startWatcherForRepo(wsId, repoId);
			} else if (node.kind === 'dir' && wsId) {
				// Lazy-load directory children on expand
				loadDirEntries(wsId, node.repoId, node.path);
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

	// Handle pending file selection from Cmd+P search
	$effect(() => {
		if (pendingFileSelection) {
			const { repoId, path } = pendingFileSelection;
			// Expand the repo node if collapsed
			const repoKey = `repo:${repoId}`;
			if (!expandedNodes.has(repoKey)) {
				expandedNodes = new Set([...expandedNodes, repoKey]);
			}
			selectTreeFile(path, repoId);
			onFileSelectionHandled();
		}
	});
	const navigateChangedFile = (delta: number): void => {
		const files = currentRepoChangedFiles;
		if (files.length === 0 || selectedChangedFileIdx < 0) return;
		const next = Math.max(0, Math.min(files.length - 1, selectedChangedFileIdx + delta));
		if (next !== selectedChangedFileIdx) {
			selectTreeFile(files[next].path, selectedRepoId!);
		}
	};
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
		const toExpand = buildExpandedRepoTreeKeysForQuery(filteredRepoFiles);
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
		dirEntryErrors = new Map();
		repoDiffMap = new Map();
		branchDiffMap = new Map();
		prFileCommentCounts = new Map();
		allPrReviewCommentsByRepo = new Map();
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
				maybeLoadBranchData(ws.id, firstRepo.id);
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

	// Helper: load branch diff + PR comments if repo has a tracked PR
	const maybeLoadBranchData = (currentWsId: string, repoId: string): void => {
		const repo = workspace?.repos.find((r) => r.id === repoId);
		const repoPr = repo?.trackedPullRequest;
		if (repoPr && repoPr.state.toLowerCase() === 'open') {
			void loadBranchDiff(currentWsId, repoId, repoPr.baseBranch, repoPr.headBranch);
			void loadAllPrReviewComments(currentWsId, repoId);
		}
	};

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

	// Load PR annotations when viewing a file in a repo with a tracked PR
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

	// Load blame when blame mode is toggled on or file changes
	$effect(() => {
		const blame = blameMode;
		const currentWsId = wsId;
		const repoId = selectedRepoId;
		const path = selectedFilePath;
		if (!blame || !currentWsId || !repoId || !path || editMode) return;
		void loadBlame(currentWsId, repoId, path);
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
							title="New file"
							onclick={() => {
								newFilePath = '';
								newFileDialogOpen = true;
							}}
						>
							<FilePlus size={12} />
						</button>
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
				<div class="urv-tree-list" tabindex="0" role="tree" onkeydown={handleTreeKeydown}>
					{#each treeNodes as node, idx (node.key)}
						{#if node.kind === 'repo'}
							{@const stats = repoChangeStats.get(node.repoId)}
							{@const prState = getRepoPrState(node.repoId)}
							<button
								type="button"
								class="urv-tree-repo"
								class:expanded={expandedNodes.has(node.key)}
								class:focused={idx === focusedNodeIndex}
								style={`--depth:${node.depth};`}
								onclick={() => toggleNode(node)}
							>
								{#if expandedNodes.has(node.key)}
									<ChevronDown size={11} />
								{:else}
									<ChevronRight size={11} />
								{/if}
								<GitBranch size={12} />
								<span class="urv-tree-label">{node.label}</span>
								{#if prState === 'open'}
									<span
										class="urv-pr-indicator urv-pr-open"
										role="button"
										tabindex="-1"
										title="View Pull Request"
										onclick={(e) => {
											e.stopPropagation();
											selectedRepoId = node.repoId;
											drawerMode = 'pr-lifecycle';
										}}
										onkeydown={() => {}}
									>
										<GitPullRequest size={10} />
									</span>
								{:else if prState === 'draft'}
									<span
										class="urv-pr-indicator urv-pr-draft"
										role="button"
										tabindex="-1"
										title="View Draft PR"
										onclick={(e) => {
											e.stopPropagation();
											selectedRepoId = node.repoId;
											drawerMode = 'pr-lifecycle';
										}}
										onkeydown={() => {}}
									>
										<GitPullRequest size={10} />
									</span>
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
							{@const commentCount = dirCommentCount.get(node.key) ?? 0}
							<button
								type="button"
								class="urv-tree-dir"
								class:expanded={expandedNodes.has(node.key)}
								class:has-changes={dirChanged}
								class:focused={idx === focusedNodeIndex}
								style={`--depth:${node.depth};`}
								onclick={() => toggleNode(node)}
							>
								{#if expandedNodes.has(node.key)}
									<ChevronDown size={11} />
								{:else}
									<ChevronRight size={11} />
								{/if}
								<span class="urv-tree-label">{node.label}</span>
								{#if commentCount > 0}
									<span
										class="urv-tree-comment-badge"
										title={`${commentCount} unresolved review thread${commentCount === 1 ? '' : 's'}`}
									>
										<MessageCircle size={10} />
										<span>{commentCount}</span>
									</span>
								{/if}
								{#if dirChanged && dirChanges > 0}
									<span class="urv-tree-dir-changes">{dirChanges}</span>
								{:else if childCounts.has(node.key)}
									<span class="urv-tree-count">{childCounts.get(node.key)}</span>
								{/if}
							</button>
							{#if expandedNodes.has(node.key) && getDirEntryError(node.repoId, node.path)}
								<div class="urv-tree-state error" style={`--depth:${node.depth + 1};`}>
									{getDirEntryError(node.repoId, node.path)}
								</div>
							{/if}
						{:else}
							{@const changed = isFileChanged(node.repoId, node.path)}
							{@const diffInfo = changed ? getFileDiffInfo(node.repoId, node.path) : undefined}
							<button
								type="button"
								class="urv-tree-file"
								class:selected={node.path === selectedFilePath && node.repoId === selectedRepoId}
								class:changed
								class:dirty={node.repoId === selectedRepoId &&
									node.path === selectedFilePath &&
									editedContent !== null}
								class:focused={idx === focusedNodeIndex}
								style={`--depth:${node.depth};`}
								title={node.path}
								onclick={() => selectTreeFile(node.path, node.repoId)}
							>
								<span class="urv-file-icon" data-icon={getRepoFileIcon(node.path)}>
									<Icon icon={getRepoFileIcon(node.path)} width="12" />
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
								{#if getFileCommentCount(node.repoId, node.path) > 0}
									{@const commentCount = getFileCommentCount(node.repoId, node.path)}
									<span
										class="urv-tree-comment-badge urv-tree-file-comments"
										title={`${commentCount} unresolved review thread${commentCount === 1 ? '' : 's'}`}
									>
										<MessageCircle size={10} />
										<span>{commentCount}</span>
									</span>
								{/if}
								<span
									class="urv-tree-file-delete"
									role="button"
									tabindex="-1"
									title="Delete file"
									onclick={(e) => {
										e.stopPropagation();
										deleteConfirmRepoId = node.repoId;
										deleteConfirmPath = node.path;
									}}
									onkeydown={() => {}}
								>
									<Trash2 size={10} />
								</span>
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
							{#if !editMode && !isChangedFile}
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

{#if newFileDialogOpen}
	<Modal title="Create New File" size="sm" onClose={() => (newFileDialogOpen = false)}>
		<div class="urv-new-file-dialog">
			<label>
				<span>File path (relative to repo root)</span>
				<input
					type="text"
					bind:value={newFilePath}
					placeholder="src/components/NewFile.ts"
					onkeydown={(e) => {
						if (e.key === 'Enter') void handleCreateFile();
					}}
				/>
			</label>
		</div>
		{#snippet footer()}
			<button type="button" class="urv-dialog-btn" onclick={() => (newFileDialogOpen = false)}
				>Cancel</button
			>
			<button
				type="button"
				class="urv-dialog-btn urv-dialog-btn-primary"
				onclick={() => void handleCreateFile()}>Create</button
			>
		{/snippet}
	</Modal>
{/if}

{#if deleteConfirmPath}
	<Modal
		title="Delete File"
		size="sm"
		onClose={() => {
			deleteConfirmPath = null;
			deleteConfirmRepoId = null;
		}}
	>
		<p class="urv-delete-confirm-text">
			Are you sure you want to delete <strong>{deleteConfirmPath}</strong>?
		</p>
		{#snippet footer()}
			<button
				type="button"
				class="urv-dialog-btn"
				onclick={() => {
					deleteConfirmPath = null;
					deleteConfirmRepoId = null;
				}}>Cancel</button
			>
			<button
				type="button"
				class="urv-dialog-btn urv-dialog-btn-danger"
				onclick={() => void handleDeleteFile()}>Delete</button
			>
		{/snippet}
	</Modal>
{/if}
