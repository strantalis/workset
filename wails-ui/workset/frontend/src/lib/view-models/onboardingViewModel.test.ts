import { beforeEach, describe, expect, it, vi } from 'vitest';
import { loadOnboardingCatalog } from './onboardingViewModel';
import { getGroup, listGroups, listRegisteredRepos } from '../api/settings';

vi.mock('../api/settings', () => ({
	getGroup: vi.fn(),
	listGroups: vi.fn(),
	listRegisteredRepos: vi.fn(),
}));

describe('onboardingViewModel', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(listGroups).mockResolvedValue([]);
		vi.mocked(getGroup).mockResolvedValue({ name: '', members: [] });
	});

	it('does not misclassify generic https repositories as TypeScript', async () => {
		vi.mocked(listRegisteredRepos).mockResolvedValue([
			{
				name: 'payments-service',
				url: 'https://github.com/acme/payments-service.git',
				default_branch: 'main',
			},
		]);

		const catalog = await loadOnboardingCatalog();
		expect(catalog.repoRegistry).toHaveLength(1);
		expect(catalog.repoRegistry[0]?.language).toBe('Repository');
	});

	it('classifies language from concrete extension hints', async () => {
		vi.mocked(listRegisteredRepos).mockResolvedValue([
			{
				name: 'jobs-runner',
				path: '/Users/sean/jobs-runner/main.py',
				default_branch: 'main',
			},
		]);

		const catalog = await loadOnboardingCatalog();
		expect(catalog.repoRegistry).toHaveLength(1);
		expect(catalog.repoRegistry[0]?.language).toBe('Python');
	});
});
