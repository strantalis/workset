import { describe, expect, it, vi } from 'vitest';
import type { Alias, Group, GroupSummary, Workspace } from '../../types';
import {
	deriveAddRepoContext,
	deriveExistingReposContext,
	deriveWorkspaceActionContext,
	getAliasSource,
	loadWorkspaceActionContext,
} from '../../services/workspaceActionContextService';

const buildWorkspace = (): Workspace => ({
	id: 'ws-1',
	name: 'alpha',
	path: '/tmp/alpha',
	archived: false,
	repos: [
		{
			id: 'repo-1',
			name: 'repo-a',
			path: '/tmp/alpha/repo-a',
			dirty: false,
			missing: false,
			diff: { added: 0, removed: 0 },
			files: [],
		},
		{
			id: 'repo-2',
			name: 'repo-b',
			path: '/tmp/alpha/repo-b',
			dirty: true,
			missing: false,
			diff: { added: 1, removed: 2 },
			files: [],
		},
	],
	pinned: false,
	pinOrder: 0,
	expanded: false,
	lastUsed: '2026-01-01T00:00:00Z',
});

describe('workspaceActionContextService', () => {
	it('derives create-mode context for preview/filter/name generation', () => {
		const aliasItems: Alias[] = [
			{ name: 'alpha-repo', url: 'git@github.com:acme/alpha-repo.git' },
			{ name: 'beta-repo', path: '/tmp/repos/beta' },
		];
		const groupItems: GroupSummary[] = [
			{ name: 'group-one', description: 'Alpha group', repo_count: 2 },
			{ name: 'group-two', description: 'Other', repo_count: 1 },
		];

		const result = deriveWorkspaceActionContext({
			primaryInput: 'git@github.com:acme/pending.git',
			directRepos: [{ url: '/tmp/repos/direct', register: true }],
			customizeName: '',
			searchQuery: 'alpha',
			aliasItems,
			groupItems,
			selectedAliases: new Set(['alpha-repo']),
			selectedGroups: new Set(['group-one']),
		});

		expect(result.detectedRepoName).toBe('pending');
		expect(result.inputIsSource).toBe(true);
		expect(result.firstDirectRepoName).toBe('direct');
		expect(result.nameSource).toBe('direct');
		expect(result.generatedName).toMatch(/^direct-/);
		expect(result.finalName).toBe(result.generatedName);
		expect(result.alternatives).toHaveLength(2);
		expect(result.alternatives.every((entry) => entry.startsWith('direct-'))).toBe(true);
		expect(result.filteredAliases).toEqual([aliasItems[0]]);
		expect(result.filteredGroups).toEqual([groupItems[0]]);
		expect(result.pendingInputWillBeAdded).toBe(true);
		expect(result.totalRepos).toBe(5);
		expect(result.selectedItems).toEqual([
			{ type: 'repo', name: 'direct', url: '/tmp/repos/direct', pending: false },
			{
				type: 'repo',
				name: 'pending',
				url: 'git@github.com:acme/pending.git',
				pending: true,
			},
			{ type: 'alias', name: 'alpha-repo', url: undefined, pending: false },
			{ type: 'group', name: 'group-one', url: undefined, pending: false },
		]);
	});

	it('prefers customized name and trims plain-name input fallback', () => {
		const withCustom = deriveWorkspaceActionContext({
			primaryInput: 'alpha workspace',
			directRepos: [],
			customizeName: 'custom-name',
			searchQuery: '',
			aliasItems: [],
			groupItems: [],
			selectedAliases: new Set(),
			selectedGroups: new Set(),
		});
		expect(withCustom.finalName).toBe('custom-name');

		const withPlain = deriveWorkspaceActionContext({
			primaryInput: '  alpha workspace  ',
			directRepos: [],
			customizeName: '',
			searchQuery: '',
			aliasItems: [],
			groupItems: [],
			selectedAliases: new Set(),
			selectedGroups: new Set(),
		});
		expect(withPlain.generatedName).toBeNull();
		expect(withPlain.finalName).toBe('alpha workspace');
		expect(withPlain.pendingInputWillBeAdded).toBe(false);
	});

	it('derives add-repo selections and existing repo list context', () => {
		const addContext = deriveAddRepoContext({
			addSource: ' /tmp/repos/new-repo ',
			selectedAliases: new Set(['alias-a']),
			selectedGroups: new Set(['group-a']),
		});
		expect(addContext).toEqual({
			hasSource: true,
			selectedItems: [
				{ type: 'repo', name: '/tmp/repos/new-repo' },
				{ type: 'alias', name: 'alias-a' },
				{ type: 'group', name: 'group-a' },
			],
			totalItems: 3,
		});

		const workspace = buildWorkspace();
		expect(deriveExistingReposContext({ mode: 'add-repo', workspace })).toEqual([
			{ name: 'repo-a' },
			{ name: 'repo-b' },
		]);
		expect(deriveExistingReposContext({ mode: 'rename', workspace })).toEqual([]);
		expect(deriveExistingReposContext({ mode: 'add-repo', workspace: null })).toEqual([]);
	});

	it('loads base context without alias/group fetch when mode does not require it', async () => {
		const workspace = buildWorkspace();
		const loadWorkspaces = vi.fn(async () => undefined);
		const listAliases = vi.fn(async (): Promise<Alias[]> => [{ name: 'unused' }]);
		const listGroups = vi.fn(
			async (): Promise<GroupSummary[]> => [{ name: 'unused', repo_count: 1 }],
		);
		const getGroup = vi.fn(async (): Promise<Group> => ({ name: 'unused', members: [] }));

		const result = await loadWorkspaceActionContext(
			{
				mode: 'rename',
				workspaceId: 'ws-1',
				repoName: 'repo-b',
			},
			{
				loadWorkspaces,
				getWorkspaces: () => [workspace],
				listAliases,
				listGroups,
				getGroup,
			},
		);

		expect(loadWorkspaces).toHaveBeenCalledTimes(1);
		expect(loadWorkspaces).toHaveBeenCalledWith(true);
		expect(listAliases).not.toHaveBeenCalled();
		expect(listGroups).not.toHaveBeenCalled();
		expect(getGroup).not.toHaveBeenCalled();
		expect(result.workspace?.id).toBe('ws-1');
		expect(result.repo?.name).toBe('repo-b');
		expect(result.renameName).toBe('alpha');
		expect(result.aliasItems).toEqual([]);
		expect(result.groupItems).toEqual([]);
		expect(result.groupDetails.size).toBe(0);
	});

	it('loads alias/group context and group details for create mode', async () => {
		const loadWorkspaces = vi.fn(async () => undefined);
		const aliases: Alias[] = [{ name: 'alias-a', path: '/tmp/repos/a' }];
		const groups: GroupSummary[] = [
			{ name: 'group-a', repo_count: 2, description: 'A' },
			{ name: 'group-b', repo_count: 1, description: 'B' },
		];
		const groupLookup = new Map<string, Group>([
			['group-a', { name: 'group-a', members: [{ repo: 'repo-a' }, { repo: 'repo-b' }] }],
			['group-b', { name: 'group-b', members: [{ repo: 'repo-c' }] }],
		]);
		const listAliases = vi.fn(async () => aliases);
		const listGroups = vi.fn(async () => groups);
		const getGroup = vi.fn(async (name: string) => groupLookup.get(name) as Group);

		const result = await loadWorkspaceActionContext(
			{
				mode: 'create',
				workspaceId: null,
				repoName: null,
			},
			{
				loadWorkspaces,
				getWorkspaces: () => [],
				listAliases,
				listGroups,
				getGroup,
			},
		);

		expect(loadWorkspaces).toHaveBeenCalledWith(true);
		expect(listAliases).toHaveBeenCalledTimes(1);
		expect(listGroups).toHaveBeenCalledTimes(1);
		expect(getGroup).toHaveBeenCalledTimes(2);
		expect(getGroup).toHaveBeenNthCalledWith(1, 'group-a');
		expect(getGroup).toHaveBeenNthCalledWith(2, 'group-b');
		expect(result.workspace).toBeNull();
		expect(result.repo).toBeNull();
		expect(result.renameName).toBe('');
		expect(result.aliasItems).toEqual(aliases);
		expect(result.groupItems).toEqual(groups);
		expect(result.groupDetails.get('group-a')).toEqual(['repo-a', 'repo-b']);
		expect(result.groupDetails.get('group-b')).toEqual(['repo-c']);
	});

	it('returns alias source from url/path with empty-string fallback', () => {
		expect(getAliasSource({ name: 'a', url: 'git@github.com:acme/a.git' })).toBe(
			'git@github.com:acme/a.git',
		);
		expect(getAliasSource({ name: 'a', path: '/tmp/repos/a' })).toBe('/tmp/repos/a');
		expect(getAliasSource({ name: 'a' })).toBe('');
	});
});
