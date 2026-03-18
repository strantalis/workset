import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, test, vi } from 'vitest';
import DocumentViewer from './DocumentViewer.svelte';
import { readWorkspaceRepoFile, searchWorkspaceRepoFiles } from '../api/repo-files';

vi.mock('../api/repo-files', () => ({
	readWorkspaceRepoFile: vi.fn(),
	searchWorkspaceRepoFiles: vi.fn(),
}));

vi.mock('../documentRender', () => ({
	renderCodeDocument: vi.fn(async () => ({
		html: '<pre class="shiki"><code><span class="line">const answer = 42;</span></code></pre>',
		containsMermaid: false,
	})),
	renderMarkdownDocument: vi.fn(async () => ({
		html: '<h1>Doc</h1><div class="ws-mermaid-diagram"><svg><text>Diagram</text></svg></div>',
		containsMermaid: true,
	})),
}));

const mockedReadWorkspaceRepoFile = vi.mocked(readWorkspaceRepoFile);
const mockedSearchWorkspaceRepoFiles = vi.mocked(searchWorkspaceRepoFiles);

describe('DocumentViewer', () => {
	afterEach(() => {
		cleanup();
		vi.clearAllMocks();
	});

	test('renders markdown documents and allows closing', async () => {
		const onClose = vi.fn();
		mockedSearchWorkspaceRepoFiles.mockResolvedValue([
			{
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: 'docs/README.md',
				isMarkdown: true,
				sizeBytes: 24,
				score: 1,
			},
		]);
		mockedReadWorkspaceRepoFile.mockResolvedValue({
			workspaceId: 'thread-alpha',
			repoId: 'thread-alpha::api',
			repoName: 'api',
			path: 'docs/README.md',
			content: '# Doc',
			isMarkdown: true,
			isBinary: false,
			isTruncated: false,
			sizeBytes: 24,
		});

		const { getByText, getByRole } = render(DocumentViewer, {
			props: {
				session: {
					workspaceId: 'thread-alpha',
					workspaceName: 'Alpha',
					repoId: 'thread-alpha::api',
					repoName: 'api',
					path: 'docs/README.md',
					openedAt: Date.now(),
				},
				onClose,
			},
		});

		await waitFor(() => expect(getByText('Doc')).toBeInTheDocument());

		await fireEvent.click(getByRole('button', { name: 'Close document viewer' }));
		expect(onClose).toHaveBeenCalledTimes(1);
	});

	test('shows a binary fallback instead of trying to render binary content', async () => {
		mockedSearchWorkspaceRepoFiles.mockResolvedValue([
			{
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: 'assets/logo.png',
				isMarkdown: false,
				sizeBytes: 128,
				score: 1,
			},
		]);
		mockedReadWorkspaceRepoFile.mockResolvedValue({
			workspaceId: 'thread-alpha',
			repoId: 'thread-alpha::api',
			repoName: 'api',
			path: 'assets/logo.png',
			content: '',
			isMarkdown: false,
			isBinary: true,
			isTruncated: false,
			sizeBytes: 128,
		});

		const { findByText } = render(DocumentViewer, {
			props: {
				session: {
					workspaceId: 'thread-alpha',
					workspaceName: 'Alpha',
					repoId: 'thread-alpha::api',
					repoName: 'api',
					path: 'assets/logo.png',
					openedAt: Date.now(),
				},
				onClose: vi.fn(),
			},
		});

		expect(
			await findByText('This file looks binary. Workset will not render binary content inline.'),
		).toBeInTheDocument();
	});

	test('shows a repo file tree and lets users switch files in place', async () => {
		mockedSearchWorkspaceRepoFiles.mockResolvedValue([
			{
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: '.github/dependabot.yml',
				isMarkdown: false,
				sizeBytes: 128,
				score: 1,
			},
			{
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: 'docs/README.md',
				isMarkdown: true,
				sizeBytes: 24,
				score: 1,
			},
		]);
		mockedReadWorkspaceRepoFile
			.mockResolvedValueOnce({
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: '.github/dependabot.yml',
				content: 'version: 2',
				isMarkdown: false,
				isBinary: false,
				isTruncated: false,
				sizeBytes: 11,
			})
			.mockResolvedValueOnce({
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: 'docs/README.md',
				content: '# Doc',
				isMarkdown: true,
				isBinary: false,
				isTruncated: false,
				sizeBytes: 24,
			});

		const { getByRole, getByText } = render(DocumentViewer, {
			props: {
				session: {
					workspaceId: 'thread-alpha',
					workspaceName: 'Alpha',
					repoId: 'thread-alpha::api',
					repoName: 'api',
					path: '.github/dependabot.yml',
					openedAt: Date.now(),
				},
				onClose: vi.fn(),
			},
		});

		await waitFor(() => expect(getByText('.github/dependabot.yml')).toBeInTheDocument());

		const repoButton = getByRole('button', { name: /^api/ });
		expect(repoButton).toBeInTheDocument();

		await fireEvent.click(repoButton);

		await waitFor(() => expect(getByText('.github')).toBeInTheDocument());

		await fireEvent.click(getByRole('button', { name: /^docs/ }));

		await waitFor(() => expect(getByRole('button', { name: /README\.md/i })).toBeInTheDocument());

		await fireEvent.click(getByRole('button', { name: /README\.md/i }));

		await waitFor(() => expect(getByText('Doc')).toBeInTheDocument());
		expect(mockedSearchWorkspaceRepoFiles).toHaveBeenCalledWith(
			'thread-alpha',
			'',
			5000,
			'thread-alpha::api',
		);
	});

	test('opens and closes the expanded mermaid overlay', async () => {
		mockedSearchWorkspaceRepoFiles.mockResolvedValue([
			{
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: 'docs/README.md',
				isMarkdown: true,
				sizeBytes: 24,
				score: 1,
			},
		]);
		mockedReadWorkspaceRepoFile.mockResolvedValue({
			workspaceId: 'thread-alpha',
			repoId: 'thread-alpha::api',
			repoName: 'api',
			path: 'docs/README.md',
			content: '# Doc',
			isMarkdown: true,
			isBinary: false,
			isTruncated: false,
			sizeBytes: 24,
		});

		const { findByLabelText, getByText, queryByLabelText } = render(DocumentViewer, {
			props: {
				session: {
					workspaceId: 'thread-alpha',
					workspaceName: 'Alpha',
					repoId: 'thread-alpha::api',
					repoName: 'api',
					path: 'docs/README.md',
					openedAt: Date.now(),
				},
				onClose: vi.fn(),
			},
		});

		await waitFor(() => expect(getByText('Doc')).toBeInTheDocument());

		await fireEvent.click(getByText('Diagram'));

		expect(await findByLabelText('Expanded Mermaid diagram')).toBeInTheDocument();

		await fireEvent.click(await findByLabelText('Close expanded diagram'));

		await waitFor(() =>
			expect(queryByLabelText('Expanded Mermaid diagram')).not.toBeInTheDocument(),
		);
	});
});
