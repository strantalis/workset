import { expect, test } from '@playwright/test';
import {
	assertWailsBindings,
	e2eWorkspaceFixture,
	ensureE2EWorkspaceFixture,
	commandPaletteShortcut,
	gotoApp,
	openMainRailView,
	selectWorkspaceFromHubByName,
} from './helpers/app-harness';

test.describe('shell navigation', () => {
	test('navigates across primary rail views', async ({ page }) => {
		await gotoApp(page);
		await assertWailsBindings(page);
		await ensureE2EWorkspaceFixture(page);
		await selectWorkspaceFromHubByName(page, e2eWorkspaceFixture.workspaceName);

		await openMainRailView(page, 'Command Center');
		await expect(page.getByRole('heading', { name: 'Command Center' })).toBeVisible();

		await openMainRailView(page, 'Engineering Cockpit');
		await expect(page.getByText('CURRENT WORKSET')).toBeVisible();

		await openMainRailView(page, 'PR Orchestration');
		await expect(page.getByRole('button', { name: /Active PRs/i })).toBeVisible();
		await expect(page.getByRole('button', { name: /Ready to PR/i })).toBeVisible();

		await openMainRailView(page, 'Skill Registry');
		await expect(page.getByRole('heading', { name: 'Skill Registry' })).toBeVisible();

		await openMainRailView(page, 'Workset Hub');
		await expect(page.getByRole('heading', { name: 'Worksets' })).toBeVisible();
	});

	test('opens command palette with keyboard shortcut', async ({ page }) => {
		await gotoApp(page);
		await page.keyboard.press(commandPaletteShortcut());
		const palette = page.getByRole('dialog', { name: 'Command palette' });
		await expect(palette).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(palette).toBeHidden();
	});

	test('opens and closes settings panel', async ({ page }) => {
		await gotoApp(page);
		await page.getByRole('button', { name: 'Settings' }).click();
		const settingsDialog = page.getByRole('dialog', { name: 'Settings' });
		await expect(settingsDialog).toBeVisible();
		await expect(settingsDialog.getByRole('heading', { name: 'Workspace' })).toBeVisible();
		await settingsDialog.getByRole('button', { name: 'Close settings' }).click();
		await expect(settingsDialog).toBeHidden();
	});
});
