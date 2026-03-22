import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import WorkspaceActionCreateForm from './WorkspaceActionCreateForm.svelte';
import * as githubApi from '../../api/github';

vi.mock('../../api/github', () => ({
	searchGitHubRepositories: vi.fn(),
}));

describe('WorkspaceActionCreateForm', () => {
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
		modeVariant: 'workset' as const,
		worksetLabel: null,
		workspaceName: 'platform-core',
		searchQuery: '',
		sourceInput: '',
		directRepos: [],
		filteredAliases: [],
		selectedAliases: new Set<string>(),
		getAliasSource: vi.fn(() => ''),
		onWorkspaceNameInput: vi.fn(),
		onSearchQueryInput: vi.fn(),
		onSourceInput: vi.fn(),
		onAddDirectRepo: vi.fn(),
		onRemoveDirectRepo: vi.fn(),
		onToggleDirectRepoRegister: vi.fn(),
		onToggleAlias: vi.fn(),
		onSubmit: vi.fn(),
	});

	test('searches GitHub repos from Add Repository input and renders suggestions', async () => {
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

		const { getByPlaceholderText, getByText } = render(WorkspaceActionCreateForm, {
			props: baseProps(),
		});
		const sourceInput = getByPlaceholderText(
			'Search catalog, GitHub, or paste URL',
		) as HTMLInputElement;

		await fireEvent.focus(sourceInput);
		await fireEvent.input(sourceInput, { target: { value: 'workset' } });
		await vi.advanceTimersByTimeAsync(260);

		expect(githubApi.searchGitHubRepositories).toHaveBeenCalledWith('workset', 8);
		await waitFor(() => {
			expect(getByText('strantalis/workset')).toBeInTheDocument();
		});
	});

	test('selects a GitHub suggestion into source input callback', async () => {
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
		const { getByPlaceholderText, getByText } = render(WorkspaceActionCreateForm, {
			props,
		});
		const sourceInput = getByPlaceholderText(
			'Search catalog, GitHub, or paste URL',
		) as HTMLInputElement;

		await fireEvent.focus(sourceInput);
		await fireEvent.input(sourceInput, { target: { value: 'workset' } });
		await vi.advanceTimersByTimeAsync(260);

		await fireEvent.mouseDown(getByText('strantalis/workset'));
		expect(props.onSourceInput).toHaveBeenCalledWith('git@github.com:strantalis/workset.git');
		expect(props.onAddDirectRepo).toHaveBeenCalled();
		expect(props.onSourceInput).toHaveBeenLastCalledWith('');
	});

	test('does not query GitHub for local path-like input', async () => {
		const { getByPlaceholderText } = render(WorkspaceActionCreateForm, {
			props: baseProps(),
		});
		const sourceInput = getByPlaceholderText(
			'Search catalog, GitHub, or paste URL',
		) as HTMLInputElement;

		await fireEvent.focus(sourceInput);
		await fireEvent.input(sourceInput, { target: { value: '/tmp/workset' } });
		await vi.advanceTimersByTimeAsync(260);

		expect(githubApi.searchGitHubRepositories).not.toHaveBeenCalled();
	});

	test('hides repo selection controls in thread mode', () => {
		const { queryByPlaceholderText, getByText } = render(WorkspaceActionCreateForm, {
			props: {
				...baseProps(),
				modeVariant: 'thread' as const,
				worksetLabel: 'Platform Core',
				workspaceName: 'oauth2-migration',
				selectedAliases: new Set(['auth-service', 'user-api']),
			},
		});

		expect(getByText('Platform Core')).toBeInTheDocument();
		expect(queryByPlaceholderText('Search catalog, GitHub, or paste URL')).not.toBeInTheDocument();
		expect(getByText('Create Thread')).toBeInTheDocument();
	});

	test('renders per-repo hook preview in thread mode', () => {
		const { getByText } = render(WorkspaceActionCreateForm, {
			props: {
				...baseProps(),
				modeVariant: 'thread' as const,
				worksetLabel: 'Platform Core',
				workspaceName: 'oauth2-migration',
				selectedAliases: new Set(['auth-service', 'user-api']),
				threadHookRows: [
					{
						repoName: 'auth-service',
						hooks: ['npm install', 'npm run build'],
						hasSource: true,
					},
					{
						repoName: 'user-api',
						hooks: [],
						hasSource: false,
					},
				],
				threadHooksLoading: false,
				threadHooksError: null,
			},
		});

		expect(getByText('Hooks')).toBeInTheDocument();
		expect(getByText('auth-service')).toBeInTheDocument();
		expect(getByText('npm install')).toBeInTheDocument();
		expect(getByText('npm run build')).toBeInTheDocument();
		expect(getByText('No source in catalog')).toBeInTheDocument();
	});
});
