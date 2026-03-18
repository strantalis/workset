import {
	deriveRepoName,
	generateAlternatives,
	generateWorkspaceName,
	isRepoSource,
} from '../names';
import type { Alias, Repo, Workspace } from '../types';

export type WorkspaceActionMode =
	| 'create'
	| 'create-thread'
	| 'rename'
	| 'add-repo'
	| 'archive'
	| 'remove-thread'
	| 'remove-repo'
	| null;

export type WorkspaceActionDirectRepo = {
	url: string;
	register: boolean;
};

export type WorkspaceActionPreviewItem = {
	type: 'repo' | 'alias';
	name: string;
	url?: string;
	pending: boolean;
};

export type WorkspaceActionAddRepoSelectedItem = {
	type: 'repo' | 'alias';
	name: string;
};

type WorkspaceActionDerivationInput = {
	primaryInput: string;
	directRepos: WorkspaceActionDirectRepo[];
	customizeName: string;
	searchQuery: string;
	aliasItems: Alias[];
	selectedAliases: Set<string>;
};

export type WorkspaceActionDerivationResult = {
	detectedRepoName: string | null;
	inputIsSource: boolean;
	firstDirectRepoName: string | null;
	firstSelectedAlias: string | null;
	nameSource: string | null;
	generatedName: string | null;
	finalName: string;
	alternatives: string[];
	filteredAliases: Alias[];
	pendingInputWillBeAdded: boolean;
	totalRepos: number;
	selectedItems: WorkspaceActionPreviewItem[];
};

type AddRepoDerivationInput = {
	addSource: string;
	selectedAliases: Set<string>;
};

export type AddRepoDerivationResult = {
	hasSource: boolean;
	selectedItems: WorkspaceActionAddRepoSelectedItem[];
	totalItems: number;
};

type ExistingReposInput = {
	mode: WorkspaceActionMode;
	workspace: Workspace | null;
	workspaces?: Workspace[];
	workspaceIds?: string[];
};

export type ExistingRepoContext = {
	name: string;
};

type LoadWorkspaceActionContextInput = {
	mode: WorkspaceActionMode;
	workspaceId: string | null;
	repoName: string | null;
};

type LoadWorkspaceActionContextDeps = {
	loadWorkspaces: (includeArchived: boolean) => Promise<void>;
	getWorkspaces: () => Workspace[];
	listRegisteredRepos: () => Promise<Alias[]>;
};

export type LoadWorkspaceActionContextResult = {
	workspace: Workspace | null;
	repo: Repo | null;
	renameName: string;
	aliasItems: Alias[];
};

export const getAliasSource = (alias: Alias): string => alias.url || alias.path || '';

export const deriveWorkspaceActionContext = (
	input: WorkspaceActionDerivationInput,
): WorkspaceActionDerivationResult => {
	const detectedRepoName = deriveRepoName(input.primaryInput);
	const inputIsSource = isRepoSource(input.primaryInput);
	const firstDirectRepoName =
		input.directRepos.length > 0 ? deriveRepoName(input.directRepos[0].url) : null;
	const firstSelectedAlias =
		input.selectedAliases.size > 0 ? Array.from(input.selectedAliases)[0] : null;
	const nameSource =
		firstDirectRepoName || (inputIsSource ? detectedRepoName : null) || firstSelectedAlias;
	const generatedName = nameSource ? generateWorkspaceName(nameSource) : null;
	const finalName = input.customizeName || generatedName || input.primaryInput.trim();
	const alternatives = nameSource ? generateAlternatives(nameSource, 2) : [];

	const normalizedSearch = input.searchQuery.toLowerCase();
	const filteredAliases = input.searchQuery
		? input.aliasItems.filter(
				(alias) =>
					alias.name.toLowerCase().includes(normalizedSearch) ||
					getAliasSource(alias).toLowerCase().includes(normalizedSearch),
			)
		: input.aliasItems;

	const pendingSource = input.primaryInput.trim();
	const pendingInputWillBeAdded =
		inputIsSource && !input.directRepos.some((entry) => entry.url === pendingSource);

	const totalRepos =
		input.directRepos.length + (pendingInputWillBeAdded ? 1 : 0) + input.selectedAliases.size;

	const selectedItems: WorkspaceActionPreviewItem[] = [
		...input.directRepos.map((entry) => ({
			type: 'repo' as const,
			name: deriveRepoName(entry.url) || entry.url,
			url: entry.url,
			pending: false,
		})),
		...(pendingInputWillBeAdded
			? [
					{
						type: 'repo' as const,
						name: detectedRepoName || pendingSource,
						url: pendingSource,
						pending: true,
					},
				]
			: []),
		...Array.from(input.selectedAliases).map((name) => ({
			type: 'alias' as const,
			name,
			url: undefined,
			pending: false,
		})),
	];

	return {
		detectedRepoName,
		inputIsSource,
		firstDirectRepoName,
		firstSelectedAlias,
		nameSource,
		generatedName,
		finalName,
		alternatives,
		filteredAliases,
		pendingInputWillBeAdded,
		totalRepos,
		selectedItems,
	};
};

export const deriveAddRepoContext = (input: AddRepoDerivationInput): AddRepoDerivationResult => {
	const source = input.addSource.trim();
	const hasSource = source.length > 0;
	const selectedItems: WorkspaceActionAddRepoSelectedItem[] = [
		...(hasSource ? [{ type: 'repo' as const, name: source }] : []),
		...Array.from(input.selectedAliases).map((name) => ({ type: 'alias' as const, name })),
	];
	return {
		hasSource,
		selectedItems,
		totalItems: selectedItems.length,
	};
};

export const deriveExistingReposContext = (input: ExistingReposInput): ExistingRepoContext[] => {
	if (input.mode !== 'add-repo' || !input.workspace) {
		return [];
	}

	const targetIds = Array.from(
		new Set((input.workspaceIds ?? []).map((id) => id.trim()).filter((id) => id.length > 0)),
	);
	if (targetIds.length <= 1 || !input.workspaces || input.workspaces.length === 0) {
		return input.workspace.repos
			.map((entry) => ({ name: entry.name }))
			.sort((left, right) => left.name.localeCompare(right.name));
	}

	const targetWorkspaces = targetIds
		.map((id) => input.workspaces?.find((entry) => entry.id === id))
		.filter((entry): entry is Workspace => entry !== undefined);
	if (targetWorkspaces.length <= 1) {
		return input.workspace.repos
			.map((entry) => ({ name: entry.name }))
			.sort((left, right) => left.name.localeCompare(right.name));
	}

	const commonRepoNames = new Set(targetWorkspaces[0].repos.map((entry) => entry.name));
	for (const target of targetWorkspaces.slice(1)) {
		const names = new Set(target.repos.map((entry) => entry.name));
		for (const candidate of Array.from(commonRepoNames)) {
			if (!names.has(candidate)) {
				commonRepoNames.delete(candidate);
			}
		}
	}

	return Array.from(commonRepoNames)
		.sort((left, right) => left.localeCompare(right))
		.map((name) => ({ name }));
};

export const loadWorkspaceActionContext = async (
	input: LoadWorkspaceActionContextInput,
	deps: LoadWorkspaceActionContextDeps,
): Promise<LoadWorkspaceActionContextResult> => {
	const requiresWorkspaceSnapshotRefresh =
		input.mode !== 'create' && input.mode !== 'create-thread';
	if (requiresWorkspaceSnapshotRefresh) {
		await deps.loadWorkspaces(true);
	}
	const current = deps.getWorkspaces();
	const workspace = input.workspaceId
		? current.find((entry) => entry.id === input.workspaceId) || null
		: null;
	const repo =
		workspace && input.repoName
			? workspace.repos.find((entry) => entry.name === input.repoName) || null
			: null;

	let aliasItems: Alias[] = [];

	if (input.mode === 'add-repo' || input.mode === 'create' || input.mode === 'create-thread') {
		aliasItems = await deps.listRegisteredRepos();
	}

	return {
		workspace,
		repo,
		renameName: input.mode === 'rename' && workspace ? workspace.name : '',
		aliasItems,
	};
};
