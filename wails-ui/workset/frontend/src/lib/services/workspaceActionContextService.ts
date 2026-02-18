import {
	deriveRepoName,
	generateAlternatives,
	generateWorkspaceName,
	isRepoSource,
} from '../names';
import type { Alias, Group, GroupSummary, Repo, Workspace } from '../types';

export type WorkspaceActionMode =
	| 'create'
	| 'rename'
	| 'add-repo'
	| 'archive'
	| 'remove-workspace'
	| 'remove-repo'
	| null;

export type WorkspaceActionDirectRepo = {
	url: string;
	register: boolean;
};

export type WorkspaceActionPreviewItem = {
	type: 'repo' | 'alias' | 'group';
	name: string;
	url?: string;
	pending: boolean;
};

export type WorkspaceActionAddRepoSelectedItem = {
	type: 'repo' | 'alias' | 'group';
	name: string;
};

type WorkspaceActionDerivationInput = {
	primaryInput: string;
	directRepos: WorkspaceActionDirectRepo[];
	customizeName: string;
	searchQuery: string;
	aliasItems: Alias[];
	groupItems: GroupSummary[];
	selectedAliases: Set<string>;
	selectedGroups: Set<string>;
};

export type WorkspaceActionDerivationResult = {
	detectedRepoName: string | null;
	inputIsSource: boolean;
	firstDirectRepoName: string | null;
	firstSelectedAlias: string | null;
	firstSelectedGroup: string | null;
	nameSource: string | null;
	generatedName: string | null;
	finalName: string;
	alternatives: string[];
	filteredAliases: Alias[];
	filteredGroups: GroupSummary[];
	pendingInputWillBeAdded: boolean;
	totalRepos: number;
	selectedItems: WorkspaceActionPreviewItem[];
};

type AddRepoDerivationInput = {
	addSource: string;
	selectedAliases: Set<string>;
	selectedGroups: Set<string>;
};

export type AddRepoDerivationResult = {
	hasSource: boolean;
	selectedItems: WorkspaceActionAddRepoSelectedItem[];
	totalItems: number;
};

type ExistingReposInput = {
	mode: WorkspaceActionMode;
	workspace: Workspace | null;
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
	listAliases: () => Promise<Alias[]>;
	listGroups: () => Promise<GroupSummary[]>;
	getGroup: (name: string) => Promise<Group>;
};

export type LoadWorkspaceActionContextResult = {
	workspace: Workspace | null;
	repo: Repo | null;
	renameName: string;
	aliasItems: Alias[];
	groupItems: GroupSummary[];
	groupDetails: Map<string, string[]>;
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
	const firstSelectedGroup =
		input.selectedGroups.size > 0 ? Array.from(input.selectedGroups)[0] : null;
	const nameSource =
		firstDirectRepoName ||
		(inputIsSource ? detectedRepoName : null) ||
		firstSelectedAlias ||
		firstSelectedGroup;
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

	const filteredGroups = input.searchQuery
		? input.groupItems.filter(
				(group) =>
					group.name.toLowerCase().includes(normalizedSearch) ||
					(group.description?.toLowerCase() || '').includes(normalizedSearch),
			)
		: input.groupItems;

	const pendingSource = input.primaryInput.trim();
	const pendingInputWillBeAdded =
		inputIsSource && !input.directRepos.some((entry) => entry.url === pendingSource);

	const totalRepos =
		input.directRepos.length +
		(pendingInputWillBeAdded ? 1 : 0) +
		input.selectedAliases.size +
		Array.from(input.selectedGroups).reduce(
			(sum, name) => sum + (input.groupItems.find((entry) => entry.name === name)?.repo_count || 0),
			0,
		);

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
		...Array.from(input.selectedGroups).map((name) => ({
			type: 'group' as const,
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
		firstSelectedGroup,
		nameSource,
		generatedName,
		finalName,
		alternatives,
		filteredAliases,
		filteredGroups,
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
		...Array.from(input.selectedGroups).map((name) => ({ type: 'group' as const, name })),
	];
	return {
		hasSource,
		selectedItems,
		totalItems: selectedItems.length,
	};
};

export const deriveExistingReposContext = (input: ExistingReposInput): ExistingRepoContext[] =>
	input.mode === 'add-repo' && input.workspace
		? input.workspace.repos.map((entry) => ({ name: entry.name }))
		: [];

export const loadWorkspaceActionContext = async (
	input: LoadWorkspaceActionContextInput,
	deps: LoadWorkspaceActionContextDeps,
): Promise<LoadWorkspaceActionContextResult> => {
	await deps.loadWorkspaces(true);
	const current = deps.getWorkspaces();
	const workspace = input.workspaceId
		? current.find((entry) => entry.id === input.workspaceId) || null
		: null;
	const repo =
		workspace && input.repoName
			? workspace.repos.find((entry) => entry.name === input.repoName) || null
			: null;

	let aliasItems: Alias[] = [];
	let groupItems: GroupSummary[] = [];
	let groupDetails = new Map<string, string[]>();

	if (input.mode === 'add-repo' || input.mode === 'create') {
		aliasItems = await deps.listAliases();
		groupItems = await deps.listGroups();
		const details = new Map<string, string[]>();
		for (const group of groupItems) {
			const fullGroup = await deps.getGroup(group.name);
			details.set(
				group.name,
				fullGroup.members.map((member) => member.repo),
			);
		}
		groupDetails = details;
	}

	return {
		workspace,
		repo,
		renameName: input.mode === 'rename' && workspace ? workspace.name : '',
		aliasItems,
		groupItems,
		groupDetails,
	};
};
