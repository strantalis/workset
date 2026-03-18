import { beforeEach, describe, expect, test, vi } from 'vitest';
import {
	listRegisteredRepos,
	registerRepo,
	restartSessiond,
	unregisterRepo,
	updateRegisteredRepo,
} from './api/settings';
import {
	ListRegisteredRepos,
	RegisterRepo,
	RestartSessiond,
	RestartSessiondWithReason,
	UnregisterRepo,
	UpdateRegisteredRepo,
} from '../../bindings/workset/app';

vi.mock('../../bindings/workset/app', () => ({
	ListRegisteredRepos: vi.fn(),
	RegisterRepo: vi.fn(),
	RestartSessiond: vi.fn(),
	RestartSessiondWithReason: vi.fn(),
	UnregisterRepo: vi.fn(),
	UpdateRegisteredRepo: vi.fn(),
}));

describe('settings API', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test('restartSessiond uses default restart when reason is missing or blank', async () => {
		vi.mocked(RestartSessiond).mockResolvedValue({ available: true, error: '', warning: '' });

		await restartSessiond();
		await restartSessiond('   ');

		expect(RestartSessiond).toHaveBeenCalledTimes(2);
		expect(RestartSessiondWithReason).not.toHaveBeenCalled();
	});

	test('restartSessiond forwards trimmed reason when provided', async () => {
		vi.mocked(RestartSessiondWithReason).mockResolvedValue({
			available: true,
			error: '',
			warning: '',
		});

		await restartSessiond('  maintenance window  ');

		expect(RestartSessiondWithReason).toHaveBeenCalledWith('maintenance window');
		expect(RestartSessiond).not.toHaveBeenCalled();
	});

	test('registered repo helpers call the expected underlying API payloads', async () => {
		vi.mocked(ListRegisteredRepos).mockResolvedValue([
			{
				name: 'workset',
				url: 'https://example/repo.git',
				path: '',
				remote: 'origin',
				default_branch: 'main',
			},
		]);

		await registerRepo('workset', 'https://example/repo.git', 'origin', 'main');
		await updateRegisteredRepo('workset', 'https://example/renamed.git', 'upstream', 'trunk');
		await unregisterRepo('workset');
		const aliases = await listRegisteredRepos();

		expect(RegisterRepo).toHaveBeenCalledWith({
			name: 'workset',
			source: 'https://example/repo.git',
			remote: 'origin',
			defaultBranch: 'main',
		});
		expect(UpdateRegisteredRepo).toHaveBeenCalledWith({
			name: 'workset',
			source: 'https://example/renamed.git',
			remote: 'upstream',
			defaultBranch: 'trunk',
		});
		expect(UnregisterRepo).toHaveBeenCalledWith('workset');
		expect(aliases).toEqual([
			{
				name: 'workset',
				url: 'https://example/repo.git',
				path: '',
				remote: 'origin',
				default_branch: 'main',
			},
		]);
	});
});
