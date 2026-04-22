import { beforeEach, describe, expect, test, vi } from 'vitest';
import {
	clearRepoFileSearchCache,
	clearWorkspaceExtraRootsCache,
	getRepoFileDefinition,
	getRepoFileHover,
	listWorkspaceExtraRoots,
	searchWorkspaceRepoFiles,
} from './repo-files';
import { ListWorkspaceExtraRoots, SearchWorkspaceRepoFiles } from '../../../bindings/workset/app';
import { Call } from '@wailsio/runtime';

vi.mock('../../../bindings/workset/app', () => ({
	SearchWorkspaceRepoFiles: vi.fn(),
	ListWorkspaceExtraRoots: vi.fn(),
	ReadWorkspaceRepoFile: vi.fn(),
}));

vi.mock('@wailsio/runtime', () => ({
	Call: {
		ByID: vi.fn(async () => undefined),
		ByName: vi.fn(),
	},
	CancellablePromise: Promise,
	Create: {
		Any: (value: unknown) => value,
		Array:
			<T>(createValue: (value: unknown) => T) =>
			(value: unknown): T[] => {
				if (!Array.isArray(value)) return [];
				return value.map((entry) => createValue(entry));
			},
		Nullable:
			<T>(createValue: (value: unknown) => T) =>
			(value: unknown): T | null => {
				if (value === null || value === undefined) return null;
				return createValue(value);
			},
		Map:
			<TKey, TValue>(createKey: (key: unknown) => TKey, createValue: (value: unknown) => TValue) =>
			(value: unknown): Record<string, TValue> => {
				if (value === null || typeof value !== 'object' || Array.isArray(value)) {
					return {};
				}
				const out: Record<string, TValue> = {};
				for (const [entryKey, entryValue] of Object.entries(value)) {
					const normalizedKey = createKey(entryKey);
					out[String(normalizedKey)] = createValue(entryValue);
				}
				return out;
			},
		Events: {},
	},
	Log: {
		Debug: vi.fn(),
		Info: vi.fn(),
		Warn: vi.fn(),
		Error: vi.fn(),
	},
	Browser: {
		OpenURL: vi.fn(async () => undefined),
	},
	Events: {
		On: vi.fn(() => () => undefined),
		Off: vi.fn(),
		Emit: vi.fn(async () => undefined),
	},
}));

const mockedSearchWorkspaceRepoFiles = vi.mocked(SearchWorkspaceRepoFiles);
const mockedListWorkspaceExtraRoots = vi.mocked(ListWorkspaceExtraRoots);
const mockedCallByName = vi.mocked(Call.ByName);

describe('searchWorkspaceRepoFiles cache', () => {
	beforeEach(() => {
		clearRepoFileSearchCache();
		clearWorkspaceExtraRootsCache();
		vi.clearAllMocks();
	});

	test('reuses a cached file index across multiple queries', async () => {
		mockedSearchWorkspaceRepoFiles.mockResolvedValue([
			{
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: 'docs/README.md',
				isMarkdown: true,
				sizeBytes: 24,
				score: 0,
			},
			{
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: 'internal/config/config.go',
				isMarkdown: false,
				sizeBytes: 128,
				score: 0,
			},
		]);

		const first = await searchWorkspaceRepoFiles('thread-alpha', 'readme', 20);
		const second = await searchWorkspaceRepoFiles('thread-alpha', 'config', 20);

		expect(first).toHaveLength(1);
		expect(second).toHaveLength(1);
		expect(mockedSearchWorkspaceRepoFiles).toHaveBeenCalledTimes(1);
		expect(mockedSearchWorkspaceRepoFiles).toHaveBeenCalledWith({
			workspaceId: 'thread-alpha',
			repoId: undefined,
			query: '',
			limit: 5000,
		});
	});

	test('supports repo-scoped cache keys', async () => {
		mockedSearchWorkspaceRepoFiles.mockResolvedValue([
			{
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: 'docs/README.md',
				isMarkdown: true,
				sizeBytes: 24,
				score: 0,
			},
		]);

		await searchWorkspaceRepoFiles('thread-alpha', '', 20, 'thread-alpha::api');
		await searchWorkspaceRepoFiles('thread-alpha', 'readme', 20, 'thread-alpha::api');

		expect(mockedSearchWorkspaceRepoFiles).toHaveBeenCalledTimes(1);
		expect(mockedSearchWorkspaceRepoFiles).toHaveBeenCalledWith({
			workspaceId: 'thread-alpha',
			repoId: 'thread-alpha::api',
			query: '',
			limit: 5000,
		});
	});

	test('caches workspace extra roots', async () => {
		mockedListWorkspaceExtraRoots.mockResolvedValue([
			{
				id: 'thread-alpha::extra::scratch',
				label: 'scratch',
				relativePath: 'scratch',
				gitDetected: false,
			},
		]);

		const first = await listWorkspaceExtraRoots('thread-alpha');
		const second = await listWorkspaceExtraRoots('thread-alpha');

		expect(first).toHaveLength(1);
		expect(second).toHaveLength(1);
		expect(mockedListWorkspaceExtraRoots).toHaveBeenCalledTimes(1);
		expect(mockedListWorkspaceExtraRoots).toHaveBeenCalledWith('thread-alpha');
	});

	test('calls the app hover API by name', async () => {
		mockedCallByName.mockResolvedValue({
			supported: true,
			available: true,
			found: true,
			header: 'fn map<T, U>(value: T): U',
		});

		const result = await getRepoFileHover({
			workspaceId: 'thread-alpha',
			repoId: 'thread-alpha::api',
			path: 'src/example.ts',
			content: 'map(foo)',
			line: 0,
			character: 2,
		});

		expect(result.found).toBe(true);
		expect(mockedCallByName).toHaveBeenCalledWith('main.App.GetRepoFileHover', {
			workspaceId: 'thread-alpha',
			repoId: 'thread-alpha::api',
			path: 'src/example.ts',
			content: 'map(foo)',
			line: 0,
			character: 2,
		});
	});

	test('calls the app definition API by name', async () => {
		mockedCallByName.mockResolvedValue({
			supported: true,
			available: true,
			found: true,
			targets: [
				{
					repoId: 'thread-alpha::api',
					path: 'src/lib.ts',
					line: 4,
					character: 16,
					endLine: 4,
					endCharacter: 22,
				},
			],
		});

		const result = await getRepoFileDefinition({
			workspaceId: 'thread-alpha',
			repoId: 'thread-alpha::api',
			path: 'src/example.ts',
			content: 'helper()',
			line: 0,
			character: 1,
		});

		expect(result.found).toBe(true);
		expect(mockedCallByName).toHaveBeenCalledWith('main.App.GetRepoFileDefinition', {
			workspaceId: 'thread-alpha',
			repoId: 'thread-alpha::api',
			path: 'src/example.ts',
			content: 'helper()',
			line: 0,
			character: 1,
		});
	});
});
