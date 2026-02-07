import { beforeEach, describe, expect, test, vi } from 'vitest';
import { createAlias, deleteAlias, listAliases, restartSessiond, updateAlias } from './api';
import {
	CreateAlias,
	DeleteAlias,
	ListAliases,
	RestartSessiond,
	RestartSessiondWithReason,
	UpdateAlias,
} from '../../wailsjs/go/main/App';

vi.mock('../../wailsjs/go/main/App', () => ({
	CreateAlias: vi.fn(),
	DeleteAlias: vi.fn(),
	ListAliases: vi.fn(),
	RestartSessiond: vi.fn(),
	RestartSessiondWithReason: vi.fn(),
	UpdateAlias: vi.fn(),
}));

describe('settings API compatibility exports', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test('restartSessiond uses default restart when reason is missing or blank', async () => {
		vi.mocked(RestartSessiond).mockResolvedValue({ available: true });

		await restartSessiond();
		await restartSessiond('   ');

		expect(RestartSessiond).toHaveBeenCalledTimes(2);
		expect(RestartSessiondWithReason).not.toHaveBeenCalled();
	});

	test('restartSessiond forwards trimmed reason when provided', async () => {
		vi.mocked(RestartSessiondWithReason).mockResolvedValue({ available: true });

		await restartSessiond('  maintenance window  ');

		expect(RestartSessiondWithReason).toHaveBeenCalledWith('maintenance window');
		expect(RestartSessiond).not.toHaveBeenCalled();
	});

	test('deprecated alias exports call the same underlying API payloads', async () => {
		vi.mocked(ListAliases).mockResolvedValue([
			{
				name: 'workset',
				url: 'https://example/repo.git',
				remote: 'origin',
				default_branch: 'main',
			},
		]);

		await createAlias('workset', 'https://example/repo.git', 'origin', 'main');
		await updateAlias('workset', 'https://example/renamed.git', 'upstream', 'trunk');
		await deleteAlias('workset');
		const aliases = await listAliases();

		expect(CreateAlias).toHaveBeenCalledWith({
			name: 'workset',
			source: 'https://example/repo.git',
			remote: 'origin',
			defaultBranch: 'main',
		});
		expect(UpdateAlias).toHaveBeenCalledWith({
			name: 'workset',
			source: 'https://example/renamed.git',
			remote: 'upstream',
			defaultBranch: 'trunk',
		});
		expect(DeleteAlias).toHaveBeenCalledWith('workset');
		expect(aliases).toEqual([
			{
				name: 'workset',
				url: 'https://example/repo.git',
				remote: 'origin',
				default_branch: 'main',
			},
		]);
	});
});
