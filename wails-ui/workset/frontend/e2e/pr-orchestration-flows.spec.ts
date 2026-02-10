import { expect, test } from '@playwright/test';
import {
	assertWailsBindings,
	e2eWorkspaceFixture,
	ensureE2EWorkspaceFixture,
	gotoApp,
	listWorkspaceSnapshots,
	openMainRailView,
	selectWorkspaceFromHubByName,
	type WorkspaceSnapshot,
} from './helpers/app-harness';

const findWorkspace = (
	snapshots: WorkspaceSnapshot[],
	name: string,
): WorkspaceSnapshot | undefined => snapshots.find((workspace) => workspace.name === name);

const expectedReadyCount = (workspace: WorkspaceSnapshot): number =>
	workspace.repos.filter(
		(repo) =>
			!repo.trackedPullRequest &&
			((repo.ahead ?? 0) > 0 || repo.dirty || (repo.files?.length ?? 0) > 0),
	).length;

test.describe('pr orchestration', () => {
	test('keeps active and ready counts aligned with API snapshot data', async ({ page }) => {
		await gotoApp(page);
		await assertWailsBindings(page);
		await ensureE2EWorkspaceFixture(page);
		const snapshots = await listWorkspaceSnapshots(page);

		const selectedWorkspaceName = await selectWorkspaceFromHubByName(
			page,
			e2eWorkspaceFixture.workspaceName,
		);
		expect(selectedWorkspaceName).toBeTruthy();
		if (!selectedWorkspaceName) return;

		const snapshot = findWorkspace(snapshots, selectedWorkspaceName);
		expect(snapshot).toBeTruthy();
		if (!snapshot) return;

		await openMainRailView(page, 'PR Orchestration');
		await expect(page.locator('.ws-badge-name')).toHaveText(selectedWorkspaceName);

		const activeButton = page.getByRole('button', { name: /Active PRs/i });
		const activeCount = Number.parseInt(await activeButton.locator('.ms-count').innerText(), 10);
		const expectedActive = snapshot.repos.filter((repo) => !!repo.trackedPullRequest).length;
		expect(activeCount).toBe(expectedActive);

		if (activeCount === 0) {
			await expect(page.getByText('No active PRs')).toBeVisible();
		} else {
			await expect(page.locator('.list .list-item')).toHaveCount(activeCount);
		}

		const readyButton = page.getByRole('button', { name: /Ready to PR/i });
		await readyButton.click();
		const readyBadge = readyButton.locator('.ms-count.ready');
		const readyCount =
			(await readyBadge.count()) > 0 ? Number.parseInt(await readyBadge.innerText(), 10) : 0;
		expect(readyCount).toBe(expectedReadyCount(snapshot));

		if (readyCount === 0) {
			await expect(page.getByText('All branches have PRs')).toBeVisible();
		} else {
			await expect(page.locator('.list .list-item')).toHaveCount(readyCount);
		}
	});

	test('uses refresh wording for checks actions', async ({ page }) => {
		await gotoApp(page);
		await assertWailsBindings(page);
		await ensureE2EWorkspaceFixture(page);
		await selectWorkspaceFromHubByName(page, e2eWorkspaceFixture.workspaceName);
		await openMainRailView(page, 'PR Orchestration');

		const activeButton = page.getByRole('button', { name: /Active PRs/i });
		const activeCount = Number.parseInt(await activeButton.locator('.ms-count').innerText(), 10);
		if (activeCount === 0) {
			return;
		}

		await page.locator('.list .list-item').first().click();
		await page.getByRole('button', { name: 'Checks' }).click();

		const checksPanel = page.locator('.checks-panel');
		await expect(
			checksPanel.getByRole('button', { name: /Refresh checks/i }).first(),
		).toBeVisible();
		await expect(checksPanel.getByText(/Re-run/i)).toHaveCount(0);
	});
});
