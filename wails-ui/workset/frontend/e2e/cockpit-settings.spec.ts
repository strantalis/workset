import { expect, test } from '@playwright/test';
import {
	assertWailsBindings,
	e2eWorkspaceFixture,
	ensureE2EWorkspaceFixture,
	gotoApp,
	listAliases,
	listGroups,
	listWorkspaceSnapshots,
	openMainRailView,
	selectWorkspaceFromHubByName,
	type WorkspaceSnapshot,
} from './helpers/app-harness';

const findWorkspace = (
	snapshots: WorkspaceSnapshot[],
	name: string,
): WorkspaceSnapshot | undefined => snapshots.find((workspace) => workspace.name === name);

test.describe('cockpit and settings', () => {
	test('shows API-backed workspace/repo context in engineering cockpit', async ({ page }) => {
		await gotoApp(page);
		await assertWailsBindings(page);
		await ensureE2EWorkspaceFixture(page);
		const snapshots = await listWorkspaceSnapshots(page);
		const selectedWorkspaceName = await selectWorkspaceFromHubByName(
			page,
			e2eWorkspaceFixture.workspaceName,
		);

		await openMainRailView(page, 'Engineering Cockpit');
		expect(selectedWorkspaceName).toBeTruthy();

		const snapshot = findWorkspace(snapshots, selectedWorkspaceName);
		expect(snapshot).toBeTruthy();
		if (!snapshot) return;

		await expect(page.locator('.workset-item.active .workset-label')).toHaveText(
			selectedWorkspaceName,
		);
		await expect(page.locator('.tree-repo')).toHaveCount(snapshot.repos.length);

		await expect(page.locator('.env-selector')).toBeDisabled();
		await expect(page.locator('.toggle input')).toBeDisabled();
		await expect(page.locator('.config-btn')).toBeDisabled();
		await expect(page.locator('.env-selector')).toContainText('coming soon');
		await expect(page.locator('.config-btn')).toContainText('coming soon');
	});

	test('navigates settings sections and reflects library counts from API', async ({ page }) => {
		await gotoApp(page);
		const [aliases, groups] = await Promise.all([listAliases(page), listGroups(page)]);

		await page.getByRole('button', { name: 'Settings' }).click();
		const dialog = page.getByRole('dialog', { name: 'Settings' });
		await expect(dialog).toBeVisible();

		const sidebar = dialog.locator('.sidebar');
		const assertSection = async (buttonLabel: string, title: string): Promise<void> => {
			await sidebar.getByRole('button', { name: buttonLabel }).click();
			await expect(dialog.locator('.content-title')).toHaveText(title);
		};

		await assertSection('Workspace', 'Workspace');
		await assertSection('Agent', 'Agent');
		await assertSection('Terminal', 'Terminal');
		await assertSection('GitHub', 'GitHub');

		await assertSection('Repo Catalog', 'Repo Catalog');
		await expect(dialog.getByText(new RegExp(`${aliases.length}\\s+repo`)).first()).toBeVisible();

		await assertSection('Templates', 'Templates');
		await expect(
			dialog.getByText(new RegExp(`${groups.length}\\s+template`)).first(),
		).toBeVisible();

		await assertSection('About', 'About');

		await dialog.getByRole('button', { name: 'Close settings' }).click();
		await expect(dialog).toBeHidden();
	});
});
