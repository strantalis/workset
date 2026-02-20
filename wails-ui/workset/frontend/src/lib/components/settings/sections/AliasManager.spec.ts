import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import AliasManager from './AliasManager.svelte';
import * as settingsService from '../../../api/settings';
import * as githubApi from '../../../api/github';

vi.mock('../../../api/settings', () => ({
	listAliases: vi.fn(),
	createAlias: vi.fn(),
	updateAlias: vi.fn(),
	deleteAlias: vi.fn(),
	openDirectoryDialog: vi.fn(),
}));

vi.mock('../../../api/github', () => ({
	searchGitHubRepositories: vi.fn(),
}));

describe('AliasManager', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.mocked(settingsService.listAliases).mockResolvedValue([]);
		vi.mocked(githubApi.searchGitHubRepositories).mockResolvedValue([]);
	});

	afterEach(() => {
		vi.useRealTimers();
		cleanup();
		vi.clearAllMocks();
	});

	test('auto-fills repo name from URL when name field is empty', async () => {
		const { getByLabelText, getByText } = render(AliasManager, {
			props: {
				onAliasCountChange: vi.fn(),
			},
		});

		await fireEvent.click(getByText('Register Repo'));

		const nameInput = getByLabelText('Name') as HTMLInputElement;
		const sourceInput = getByLabelText(
			'Source (URL/path or GitHub repo search)',
		) as HTMLInputElement;

		expect(nameInput).toHaveValue('');
		await fireEvent.input(sourceInput, {
			target: { value: 'https://github.com/acme/widget-service.git' },
		});

		expect(nameInput).toHaveValue('widget-service');
	});

	test('does not overwrite a user-provided name when source URL is entered', async () => {
		const { getByLabelText, getByText } = render(AliasManager, {
			props: {
				onAliasCountChange: vi.fn(),
			},
		});

		await fireEvent.click(getByText('Register Repo'));

		const nameInput = getByLabelText('Name') as HTMLInputElement;
		const sourceInput = getByLabelText(
			'Source (URL/path or GitHub repo search)',
		) as HTMLInputElement;

		await fireEvent.input(nameInput, {
			target: { value: 'custom-name' },
		});
		await fireEvent.input(sourceInput, {
			target: { value: 'https://github.com/acme/widget-service.git' },
		});

		expect(nameInput).toHaveValue('custom-name');
	});

	test('searches remote repos and applies selected suggestion to source, name, and branch', async () => {
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

		const { getByLabelText, getByText } = render(AliasManager, {
			props: {
				onAliasCountChange: vi.fn(),
			},
		});

		await fireEvent.click(getByText('Register Repo'));

		const nameInput = getByLabelText('Name') as HTMLInputElement;
		const sourceInput = getByLabelText(
			'Source (URL/path or GitHub repo search)',
		) as HTMLInputElement;
		const branchInput = getByLabelText('Default branch') as HTMLInputElement;

		await fireEvent.input(sourceInput, { target: { value: 'workset' } });
		await vi.advanceTimersByTimeAsync(260);

		expect(githubApi.searchGitHubRepositories).toHaveBeenCalledWith('workset', 8);

		const suggestion = getByText('strantalis/workset');
		await fireEvent.mouseDown(suggestion);

		expect(sourceInput).toHaveValue('git@github.com:strantalis/workset.git');
		expect(nameInput).toHaveValue('workset');
		expect(branchInput).toHaveValue('main');
	});

	test('does not search remote repos for local path-like source', async () => {
		const { getByLabelText, getByText } = render(AliasManager, {
			props: {
				onAliasCountChange: vi.fn(),
			},
		});

		await fireEvent.click(getByText('Register Repo'));
		const sourceInput = getByLabelText(
			'Source (URL/path or GitHub repo search)',
		) as HTMLInputElement;

		await fireEvent.input(sourceInput, { target: { value: '/tmp/workset' } });
		await vi.advanceTimersByTimeAsync(260);

		expect(githubApi.searchGitHubRepositories).not.toHaveBeenCalled();
	});

	test('does not search remote repos while editing an existing repo', async () => {
		vi.mocked(settingsService.listAliases).mockResolvedValue([
			{
				name: 'workset',
				url: 'https://github.com/strantalis/workset.git',
				default_branch: 'main',
			},
		]);

		const { getByLabelText, findByTitle } = render(AliasManager, {
			props: {
				onAliasCountChange: vi.fn(),
			},
		});

		const editButton = await findByTitle('Edit');
		await fireEvent.click(editButton);

		const sourceInput = getByLabelText(
			'Source (URL/path or GitHub repo search)',
		) as HTMLInputElement;
		await fireEvent.input(sourceInput, { target: { value: 'workset' } });
		await vi.advanceTimersByTimeAsync(260);

		expect(githubApi.searchGitHubRepositories).not.toHaveBeenCalled();
	});

	test('shows source search guidance before typing', async () => {
		const { getByLabelText, getByText } = render(AliasManager, {
			props: {
				onAliasCountChange: vi.fn(),
			},
		});

		await fireEvent.click(getByText('Register Repo'));
		expect(
			getByText('Tip: type 2+ characters to search your GitHub repos, or paste a URL/path.'),
		).toBeInTheDocument();

		const sourceInput = getByLabelText(
			'Source (URL/path or GitHub repo search)',
		) as HTMLInputElement;
		await fireEvent.focus(sourceInput);

		expect(getByText('Start typing to search GitHub repositories.')).toBeInTheDocument();
	});

	test('shows GitHub auth guidance when remote search fails due to auth', async () => {
		vi.mocked(githubApi.searchGitHubRepositories).mockRejectedValueOnce(
			new Error('github auth required'),
		);

		const { getByLabelText, getByText } = render(AliasManager, {
			props: {
				onAliasCountChange: vi.fn(),
			},
		});

		await fireEvent.click(getByText('Register Repo'));
		const sourceInput = getByLabelText(
			'Source (URL/path or GitHub repo search)',
		) as HTMLInputElement;

		await fireEvent.input(sourceInput, { target: { value: 'workset' } });
		await vi.advanceTimersByTimeAsync(260);

		await waitFor(() => {
			expect(
				getByText('Connect GitHub in Settings -> GitHub authentication to search.'),
			).toBeInTheDocument();
		});
	});
});
