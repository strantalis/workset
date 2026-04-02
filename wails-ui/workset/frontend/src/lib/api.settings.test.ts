import { beforeEach, describe, expect, test, vi } from 'vitest';
import {
	listRegisteredRepos,
	registerRepo,
	unregisterRepo,
	updateRegisteredRepo,
} from './api/settings';
import {
	ListRegisteredRepos,
	RegisterRepo,
	UnregisterRepo,
	UpdateRegisteredRepo,
} from '../../bindings/workset/app';

vi.mock('../../bindings/workset/app', () => ({
	ListRegisteredRepos: vi.fn(),
	RegisterRepo: vi.fn(),
	UnregisterRepo: vi.fn(),
	UpdateRegisteredRepo: vi.fn(),
}));

describe('settings API', () => {
	beforeEach(() => {
		vi.clearAllMocks();
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
