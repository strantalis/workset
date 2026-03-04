/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import WorkspaceActionAddRepoForm from './WorkspaceActionAddRepoForm.svelte';
import * as githubApi from '../../api/github';
import type { Alias } from '../../types';

vi.mock('../../api/github', () => ({
	searchGitHubRepositories: vi.fn(),
}));

const aliases: Alias[] = [{ name: 'repo-alias', path: '/tmp/repos/repo-alias' }];

describe('WorkspaceActionAddRepoForm', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.mocked(githubApi.searchGitHubRepositories).mockResolvedValue([]);
	});

	afterEach(() => {
		vi.useRealTimers();
		cleanup();
		vi.clearAllMocks();
	});

	const baseProps = () => ({
		loading: false,
		aliasItems: aliases,
		searchQuery: '',
		addSource: '',
		filteredAliases: aliases,
		selectedAliases: new Set<string>(),
		existingRepos: [{ name: 'repo-existing' }],
		addRepoSelectedItems: [{ type: 'alias' as const, name: 'repo-alias' }],
		addRepoTotalItems: 1,
		worksetName: 'test-workset',
		getAliasSource: (alias: Alias) => alias.path || alias.url || '',
		onSearchQueryInput: vi.fn(),
		onAddSourceInput: vi.fn(),
		onBrowse: vi.fn(),
		onToggleAlias: vi.fn(),
		onRemoveAlias: vi.fn(),
		onSubmit: vi.fn(),
	});

	test('renders seamless non-tabbed add flow and catalog interactions', async () => {
		const props = baseProps();
		const { container, getByText, getByPlaceholderText, queryByText } = render(
			WorkspaceActionAddRepoForm,
			{
				props,
			},
		);

		expect(queryByText('Direct')).not.toBeInTheDocument();
		expect(queryByText('Groups (1)')).not.toBeInTheDocument();

		const searchInput = getByPlaceholderText(
			'Search catalog/GitHub, or paste repo URL/path (Enter)',
		) as HTMLInputElement;
		await fireEvent.input(searchInput, { target: { value: 'repo' } });
		expect(props.onSearchQueryInput).toHaveBeenCalledWith('repo');

		const aliasItem = container.querySelector('.registry-item') as HTMLButtonElement;
		await fireEvent.click(aliasItem);
		expect(props.onToggleAlias).toHaveBeenCalledWith('repo-alias');

		await fireEvent.click(getByText('Continue').closest('button') as HTMLButtonElement);
		expect(props.onSubmit).toHaveBeenCalledTimes(1);
	});

	test('searches GitHub and commits suggestion source', async () => {
		vi.mocked(githubApi.searchGitHubRepositories).mockResolvedValue([
			{
				name: 'workset',
				fullName: 'strantalis/workset',
				owner: 'strantalis',
				defaultBranch: 'main',
				cloneUrl: 'https://github.com/strantalis/workset.git',
				sshUrl: 'git@github.com:strantalis/workset.git',
				private: false,
				archived: false,
				host: 'github.com',
			},
		]);

		const props = baseProps();
		props.filteredAliases = [];
		const { getByPlaceholderText, getByText } = render(WorkspaceActionAddRepoForm, {
			props,
		});
		const searchInput = getByPlaceholderText(
			'Search catalog/GitHub, or paste repo URL/path (Enter)',
		) as HTMLInputElement;

		await fireEvent.input(searchInput, { target: { value: 'workset' } });
		await vi.advanceTimersByTimeAsync(260);

		expect(githubApi.searchGitHubRepositories).toHaveBeenCalledWith('workset', 8);
		await waitFor(() => {
			expect(getByText('strantalis/workset')).toBeInTheDocument();
		});

		await fireEvent.click(getByText('strantalis/workset').closest('button') as HTMLButtonElement);
		expect(props.onAddSourceInput).toHaveBeenCalledWith('git@github.com:strantalis/workset.git');
		expect(props.onSearchQueryInput).toHaveBeenCalledWith('');
	});

	test('adds a direct source on Enter without Add button', async () => {
		const props = baseProps();
		const { getByPlaceholderText, queryByRole } = render(WorkspaceActionAddRepoForm, {
			props,
		});
		const searchInput = getByPlaceholderText(
			'Search catalog/GitHub, or paste repo URL/path (Enter)',
		) as HTMLInputElement;
		await fireEvent.input(searchInput, {
			target: { value: 'git@github.com:strantalis/platform.git' },
		});
		await fireEvent.keyDown(searchInput, { key: 'Enter' });
		expect(queryByRole('button', { name: 'Add' })).not.toBeInTheDocument();
		expect(props.onAddSourceInput).toHaveBeenCalledWith('git@github.com:strantalis/platform.git');
	});

	test('disables continue when no items are selected', () => {
		const props = baseProps();
		props.addRepoSelectedItems = [];
		props.addRepoTotalItems = 0;

		const { getByText } = render(WorkspaceActionAddRepoForm, {
			props,
		});

		const submitButton = getByText('Continue').closest('button') as HTMLButtonElement;
		expect(submitButton).toBeDisabled();
	});

	test('enables continue when a valid direct source is pending', async () => {
		const props = baseProps();
		props.addRepoSelectedItems = [];
		props.addRepoTotalItems = 0;

		const { getByPlaceholderText, getByText } = render(WorkspaceActionAddRepoForm, {
			props,
		});
		const searchInput = getByPlaceholderText(
			'Search catalog/GitHub, or paste repo URL/path (Enter)',
		) as HTMLInputElement;
		await fireEvent.input(searchInput, {
			target: { value: 'git@github.com:strantalis/platform.git' },
		});

		const submitButton = getByText('Continue').closest('button') as HTMLButtonElement;
		expect(submitButton).not.toBeDisabled();
	});
});
