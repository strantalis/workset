import { expect, test, type Page } from '@playwright/test';
import {
	assertWailsBindings,
	e2eWorkspaceFixture,
	ensureE2EWorkspaceFixture,
	gotoApp,
	listWorkspaceSnapshots,
	openWorksetHub,
	readPillCount,
	readStatCard,
	selectWorkspaceFromHubByName,
	type WorkspaceSnapshot,
} from './helpers/app-harness';

const uniqueRepoCount = (snapshots: WorkspaceSnapshot[]): number =>
	new Set(snapshots.flatMap((workspace) => workspace.repos.map((repo) => repo.name.toLowerCase())))
		.size;

const openPrCount = (workspace: WorkspaceSnapshot): number =>
	workspace.repos.filter((repo) => repo.trackedPullRequest?.state.toLowerCase() === 'open').length;

const dirtyRepoCount = (snapshots: WorkspaceSnapshot[]): number =>
	snapshots.flatMap((workspace) => workspace.repos).filter((repo) => repo.dirty).length;

const findSnapshotByName = (
	snapshots: WorkspaceSnapshot[],
	workspaceName: string,
): WorkspaceSnapshot | undefined => snapshots.find((workspace) => workspace.name === workspaceName);

const assertRepoMetadataCard = async (page: Page, workspace: WorkspaceSnapshot): Promise<void> => {
	if (workspace.repos.length === 0) return;
	const repo = workspace.repos[0];
	const expectedBranch = repo.currentBranch || repo.defaultBranch || 'main';

	const repoCard = page.locator('.repo-card').filter({ hasText: repo.name }).first();
	await expect(repoCard).toBeVisible();
	await expect(repoCard.locator('.repo-branch')).toHaveText(expectedBranch);
	await expect(repoCard.locator('.meta-pair').first()).toContainText(String(repo.ahead ?? 0));
	await expect(repoCard.locator('.meta-pair').nth(1)).toContainText(String(repo.behind ?? 0));
};

test.describe('workset hub and command center', () => {
	test('renders workset hub stat pills from API snapshots', async ({ page }) => {
		await gotoApp(page);
		await assertWailsBindings(page);
		await ensureE2EWorkspaceFixture(page);
		const snapshots = await listWorkspaceSnapshots(page);
		await openWorksetHub(page);
		expect(snapshots.length).toBeGreaterThan(0);

		expect(await readPillCount(page, 'Worksets')).toBe(snapshots.length);
		expect(await readPillCount(page, 'Repos')).toBe(uniqueRepoCount(snapshots));
		expect(await readPillCount(page, 'Open PRs')).toBe(
			snapshots.reduce((acc, workspace) => acc + openPrCount(workspace), 0),
		);
		expect(await readPillCount(page, 'Pinned')).toBe(
			snapshots.filter((workspace) => workspace.pinned).length,
		);

		const totalDirty = dirtyRepoCount(snapshots);
		if (totalDirty > 0) {
			expect(await readPillCount(page, 'Dirty')).toBe(totalDirty);
		} else {
			await expect(page.locator('.stat-pill').filter({ hasText: 'Dirty' })).toHaveCount(0);
		}
	});

	test('keeps command center stats and repo metadata aligned with snapshot API', async ({
		page,
	}) => {
		await gotoApp(page);
		await assertWailsBindings(page);
		await ensureE2EWorkspaceFixture(page);
		const snapshots = await listWorkspaceSnapshots(page);

		const selectedWorkspaceName = await selectWorkspaceFromHubByName(
			page,
			e2eWorkspaceFixture.workspaceName,
		);
		expect(selectedWorkspaceName).toBeTruthy();

		const workspace = findSnapshotByName(snapshots, selectedWorkspaceName ?? '');
		expect(workspace).toBeTruthy();
		if (!workspace) return;

		await expect(page.locator('.panel-title')).toHaveText(workspace.name);
		expect(await readStatCard(page, 'Linked Repos')).toBe(workspace.repos.length);
		expect(await readStatCard(page, 'Open PRs')).toBe(openPrCount(workspace));

		await assertRepoMetadataCard(page, workspace);
	});
});
