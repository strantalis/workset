import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import SkillMarketplacePanel from './SkillMarketplacePanel.svelte';
import type { MarketplaceSkill, SkillInfo } from '../../api/skills';

const apiMocks = vi.hoisted(() => ({
	getSkill: vi.fn(),
	searchMarketplaceSkills: vi.fn(),
	getMarketplaceSkillMetadata: vi.fn(),
	getMarketplaceSkillContent: vi.fn(),
	installMarketplaceSkill: vi.fn(),
	attachSkillMarketplaceSource: vi.fn(),
}));

vi.mock('../../api/skills', () => apiMocks);

const buildMarketplaceSkill = (overrides: Partial<MarketplaceSkill> = {}): MarketplaceSkill => ({
	provider: 'skills.sh',
	externalId: 'anthropics/skills/frontend-design',
	name: 'frontend-design',
	description: 'Frontend design skill',
	sourceRepo: 'anthropics/skills',
	sourceUrl: 'https://skills.sh/anthropics/skills/frontend-design',
	rawSkillUrl:
		'https://raw.githubusercontent.com/anthropics/skills/main/skills/frontend-design/SKILL.md',
	installCount: 1200,
	verified: true,
	trustScore: 9.4,
	benchmarkScore: 95,
	relevance: 91,
	...overrides,
});

const buildInstalledSkill = (overrides: Partial<SkillInfo> = {}): SkillInfo => ({
	name: 'frontend-design',
	description: 'Frontend design skill',
	dirName: 'frontend-design',
	scope: 'project',
	tools: ['agents'],
	path: '/tmp/workspace/.agents/skills/frontend-design/SKILL.md',
	marketplace: {
		provider: 'skills.sh',
		externalId: 'anthropics/skills/frontend-design',
		sourceRepo: 'anthropics/skills',
	},
	...overrides,
});

const deferred = <T>() => {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
};

describe('SkillMarketplacePanel', () => {
	beforeEach(() => {
		apiMocks.searchMarketplaceSkills.mockReset();
		apiMocks.getSkill.mockReset();
		apiMocks.getMarketplaceSkillMetadata.mockReset();
		apiMocks.getMarketplaceSkillContent.mockReset();
		apiMocks.installMarketplaceSkill.mockReset();
		apiMocks.attachSkillMarketplaceSource.mockReset();
	});

	afterEach(() => {
		cleanup();
	});

	test('searches, previews, and installs a marketplace skill', async () => {
		const onInstalled = vi.fn();
		const skill = buildMarketplaceSkill();
		const installedSkill = buildInstalledSkill();

		apiMocks.searchMarketplaceSkills.mockResolvedValue([skill]);
		apiMocks.getSkill.mockResolvedValue({
			...installedSkill,
			content: `---
name: frontend-design
description: Frontend design skill
---

# Frontend Design`,
		});
		apiMocks.getMarketplaceSkillMetadata.mockResolvedValue(skill);
		apiMocks.getMarketplaceSkillContent.mockResolvedValue({
			skill,
			content: `---
name: frontend-design
description: Frontend design skill
---

# Frontend Design`,
		});
		apiMocks.installMarketplaceSkill.mockResolvedValue(installedSkill);

		const { getByPlaceholderText, getByRole, findAllByText } = render(SkillMarketplacePanel, {
			props: {
				workspaceId: 'ws-1',
				onInstalled,
			},
		});

		const searchInput = getByPlaceholderText('Search skills.sh...');
		await fireEvent.input(searchInput, {
			target: { value: 'frontend' },
		});
		await fireEvent.keyDown(searchInput, { key: 'Enter' });

		await findAllByText('frontend-design');
		await waitFor(() =>
			expect(apiMocks.getMarketplaceSkillContent).toHaveBeenCalledWith(skill, 'ws-1'),
		);

		await fireEvent.click(getByRole('button', { name: 'Install' }));

		await waitFor(() =>
			expect(apiMocks.installMarketplaceSkill).toHaveBeenCalledWith(
				{
					skill,
					scope: 'project',
					dirName: 'frontend-design',
					tools: ['agents'],
				},
				'ws-1',
			),
		);
		await waitFor(() =>
			expect(onInstalled).toHaveBeenCalledWith({
				installedSkill,
				message: 'Installed frontend-design to workset.',
			}),
		);
	});

	test('renders audit data in the security disclosure panel', async () => {
		const searchSkill = buildMarketplaceSkill({
			auditSummaries: [],
			repoVerified: null,
			weeklyInstalls: null,
			githubStars: null,
			firstSeen: null,
		});
		const enrichedSkill = buildMarketplaceSkill({
			auditSummaries: [
				{
					provider: 'Gen Agent Trust Hub',
					status: 'Pass',
					detailUrl: 'https://skills.sh/anthropics/skills/frontend-design',
				},
				{
					provider: 'Socket',
					status: '0 alerts',
					detailUrl: 'https://skills.sh/anthropics/skills/frontend-design',
				},
				{
					provider: 'Snyk',
					status: 'Low Risk',
					detailUrl: 'https://skills.sh/anthropics/skills/frontend-design',
				},
			],
			repoVerified: true,
			weeklyInstalls: 2500,
			githubStars: 86000,
			firstSeen: 'Jan 19, 2026',
		});

		apiMocks.searchMarketplaceSkills.mockResolvedValue([searchSkill]);
		apiMocks.getSkill.mockResolvedValue({
			...buildInstalledSkill(),
			content: `---
name: frontend-design
description: Frontend design skill
---

# Frontend Design`,
		});
		apiMocks.getMarketplaceSkillMetadata.mockResolvedValue(enrichedSkill);
		apiMocks.getMarketplaceSkillContent.mockResolvedValue({
			skill: enrichedSkill,
			content: `---
name: frontend-design
description: Frontend design skill
---

# Frontend Design`,
		});

		const { findByText, findAllByText, getByText } = render(SkillMarketplacePanel, {
			props: {
				workspaceId: 'ws-1',
			},
		});

		// The trust chip on the card shows aggregate trust
		await findAllByText('frontend-design');

		// Wait for detail hydration
		await waitFor(() => expect(apiMocks.getMarketplaceSkillContent).toHaveBeenCalled());

		// Verified org badge should be visible in the title row
		expect(getByText('Verified org')).toBeInTheDocument();

		// Security disclosure is collapsed by default — expand it
		const securityTrigger = await findByText('Security details');
		await fireEvent.click(securityTrigger);

		// Now audit details should be visible
		expect((await findAllByText('Gen Agent Trust Hub')).length).toBeGreaterThan(0);
		expect((await findAllByText('Socket')).length).toBeGreaterThan(0);
		expect((await findAllByText('Snyk')).length).toBeGreaterThan(0);
	});

	test('marks marketplace skills already installed with badge in detail pane', async () => {
		const installedWorksetSkill = buildInstalledSkill();
		const installedGlobalSkill = buildInstalledSkill({
			scope: 'global',
			path: '/tmp/.agents/skills/frontend-design/SKILL.md',
		});
		const skill = buildMarketplaceSkill();

		apiMocks.searchMarketplaceSkills.mockResolvedValue([skill]);
		apiMocks.getSkill.mockResolvedValue({
			...installedWorksetSkill,
			content: `---
name: frontend-design
description: Frontend design skill
---

# Frontend Design`,
		});
		apiMocks.getMarketplaceSkillMetadata.mockResolvedValue(skill);
		apiMocks.getMarketplaceSkillContent.mockResolvedValue({
			skill,
			content: `---
name: frontend-design
description: Frontend design skill
---

# Frontend Design`,
		});

		const { findAllByText } = render(SkillMarketplacePanel, {
			props: {
				workspaceId: 'ws-1',
				installedSkills: [installedWorksetSkill, installedGlobalSkill],
			},
		});

		// Installed badges appear in the detail pane title row
		expect((await findAllByText('Workset installed')).length).toBeGreaterThan(0);
		expect((await findAllByText('Global installed')).length).toBeGreaterThan(0);
	});

	test('does not mark same-name skills from different repos as installed', async () => {
		const installedSkill = buildInstalledSkill({
			marketplace: {
				provider: 'skills.sh',
				externalId: 'different-org/frontend-design',
				sourceRepo: 'different-org/skills',
			},
		});
		const skill = buildMarketplaceSkill();

		apiMocks.searchMarketplaceSkills.mockResolvedValue([skill]);
		apiMocks.getSkill.mockResolvedValue({
			...installedSkill,
			content: `---
name: frontend-design
description: Frontend design skill
---

# Local Frontend Design`,
		});
		apiMocks.getMarketplaceSkillMetadata.mockResolvedValue(skill);
		apiMocks.getMarketplaceSkillContent.mockResolvedValue({
			skill,
			content: `---
name: frontend-design
description: Frontend design skill
---

# Frontend Design`,
		});

		const { queryByText, findAllByText } = render(SkillMarketplacePanel, {
			props: {
				workspaceId: 'ws-1',
				installedSkills: [installedSkill],
			},
		});

		await findAllByText('frontend-design');
		expect(queryByText('Workset installed')).toBeNull();
		expect(queryByText('Global installed')).toBeNull();
	});

	test('ignores out-of-order marketplace search responses', async () => {
		const staleSearch = deferred<MarketplaceSkill[]>();
		const freshSearch = deferred<MarketplaceSkill[]>();
		const staleSkill = buildMarketplaceSkill({
			externalId: 'old-org/old-skill',
			name: 'stale-skill',
			sourceRepo: 'old-org/skills',
			sourceUrl: 'https://skills.sh/old-org/old-skill',
			rawSkillUrl:
				'https://raw.githubusercontent.com/old-org/skills/main/skills/stale-skill/SKILL.md',
		});
		const freshSkill = buildMarketplaceSkill({
			externalId: 'new-org/react-review',
			name: 'react-review',
			sourceRepo: 'new-org/skills',
			sourceUrl: 'https://skills.sh/new-org/react-review',
			rawSkillUrl:
				'https://raw.githubusercontent.com/new-org/skills/main/skills/react-review/SKILL.md',
		});

		apiMocks.searchMarketplaceSkills.mockImplementation(({ query }: { query: string }) => {
			if (query === 'frontend') return staleSearch.promise;
			if (query === 'react') return freshSearch.promise;
			return Promise.resolve([]);
		});
		apiMocks.getMarketplaceSkillMetadata.mockImplementation((skill: MarketplaceSkill) =>
			Promise.resolve(skill),
		);
		apiMocks.getMarketplaceSkillContent.mockImplementation((skill: MarketplaceSkill) =>
			Promise.resolve({
				skill,
				content: `---\nname: ${skill.name}\ndescription: ${skill.description}\n---\n\n# ${skill.name}`,
			}),
		);

		const { getByPlaceholderText, findAllByText, queryByText } = render(SkillMarketplacePanel, {
			props: {
				workspaceId: 'ws-1',
			},
		});

		const searchInput = getByPlaceholderText('Search skills.sh...');
		await fireEvent.input(searchInput, {
			target: { value: 'react' },
		});
		await fireEvent.keyDown(searchInput, { key: 'Enter' });

		freshSearch.resolve([freshSkill]);
		await findAllByText('react-review');

		staleSearch.resolve([staleSkill]);
		await waitFor(() => expect(queryByText('stale-skill')).toBeNull());
	});
});
