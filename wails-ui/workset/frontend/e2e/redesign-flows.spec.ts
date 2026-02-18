import { expect, test } from '@playwright/test';
import {
	assertWailsBindings,
	e2eWorkspaceFixture,
	ensureE2EWorkspaceFixture,
	gotoApp,
	listWorkspaceSnapshots,
	selectWorkspaceFromHubByName,
	type WorkspaceSnapshot,
} from './helpers/app-harness';

const findWorkspace = (
	snapshots: WorkspaceSnapshot[],
	name: string,
): WorkspaceSnapshot | undefined => snapshots.find((workspace) => workspace.name === name);

test.describe('redesign core flows', () => {
	test('opens command palette from context bar', async ({ page }) => {
		await gotoApp(page);
		await assertWailsBindings(page);
		await ensureE2EWorkspaceFixture(page);
		await selectWorkspaceFromHubByName(page, e2eWorkspaceFixture.workspaceName);
		await page.getByRole('button', { name: /Command palette/i }).click();
		await expect(page.getByRole('dialog', { name: 'Command palette' })).toBeVisible();
	});

	test('routes repo-card clicks through current command-center behavior (no legacy repo-diff route)', async ({
		page,
	}) => {
		await gotoApp(page);
		await assertWailsBindings(page);
		await ensureE2EWorkspaceFixture(page);
		const snapshots = await listWorkspaceSnapshots(page);
		const workspaceName = await selectWorkspaceFromHubByName(
			page,
			e2eWorkspaceFixture.workspaceName,
		);
		const workspaceSnapshot = findWorkspace(snapshots, workspaceName);
		expect(workspaceSnapshot).toBeTruthy();
		if (!workspaceSnapshot || workspaceSnapshot.repos.length === 0) return;

		const repo = workspaceSnapshot.repos[0];
		const repoCard = page.locator('.repo-card').filter({ hasText: repo.name }).first();
		await expect(repoCard).toBeVisible();
		await repoCard.locator('.repo-header').click();

		if (repo.dirty) {
			await expect(repoCard).toHaveClass(/expanded/);
			await expect(repoCard.locator('.expanded-body')).toBeVisible();
		} else if (repo.trackedPullRequest || (repo.ahead ?? 0) > 0) {
			await expect(page.getByRole('button', { name: /Active PRs/i })).toBeVisible();
			await expect(page.locator('.ws-badge-name')).toHaveText(workspaceName);
		} else {
			await expect(page.getByRole('heading', { name: 'Command Center' })).toBeVisible();
			await expect(page.locator('.expanded-body')).toHaveCount(0);
		}

		await expect(page.getByRole('button', { name: 'Back to terminal' })).toHaveCount(0);
	});

	test('loads onboarding templates from API-backed catalog', async ({ page }) => {
		await gotoApp(page);
		await page.getByRole('complementary').getByRole('button', { name: 'New Workset' }).click();
		await expect(page.getByRole('heading', { name: 'Create Workset' })).toBeVisible();

		await page
			.getByRole('textbox', { name: 'Workset Name' })
			.fill(`playwright-redesign-${Date.now()}`);
		await page.getByRole('button', { name: 'Continue' }).click();
		await page.getByRole('button', { name: /From Template/i }).click();

		const noTemplates = page.getByText('No templates found. Add templates in Settings.');
		await expect(noTemplates).toHaveCount(0);

		const templateRows = page.locator('.template-row');
		await expect(templateRows.first()).toBeVisible();
		await templateRows.first().click();
		await page.getByRole('button', { name: 'Next Step' }).click();

		const reviewRepoRows = page.locator('.review-repo-header');
		await expect(reviewRepoRows.first()).toBeVisible();
	});

	test('does not prefill onboarding workspace name from active workspace', async ({ page }) => {
		await gotoApp(page);
		await assertWailsBindings(page);
		await ensureE2EWorkspaceFixture(page);
		await selectWorkspaceFromHubByName(page, e2eWorkspaceFixture.workspaceName);
		await page.getByRole('complementary').getByRole('button', { name: 'New Workset' }).click();
		await expect(page.getByRole('heading', { name: 'Create Workset' })).toBeVisible();
		await expect(page.getByRole('textbox', { name: 'Workset Name' })).toHaveValue('');
	});
});
