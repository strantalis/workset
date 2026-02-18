import { expect, test } from '@playwright/test';
import {
	assertWailsBindings,
	closeWorkspacePopoutIfOpen,
	e2eWorkspaceFixture,
	ensureE2EWorkspaceFixture,
	gotoApp,
	gotoWorkspacePopoutView,
	openWorksetHub,
} from './helpers/app-harness';

test.describe('workspace popout', () => {
	test.beforeEach(async ({ page }) => {
		await gotoApp(page);
		await assertWailsBindings(page);
		await ensureE2EWorkspaceFixture(page);
		await closeWorkspacePopoutIfOpen(page, e2eWorkspaceFixture.workspaceName);
	});

	test('toggles workspace popout state directly from workset card action icon', async ({
		page,
	}) => {
		await openWorksetHub(page);
		const workspaceEntry = page
			.locator('.workset-card, .list-row', { hasText: e2eWorkspaceFixture.workspaceName })
			.first();

		const openTrigger = workspaceEntry.getByRole('button', { name: 'Open workspace popout' });
		await expect(openTrigger).toBeVisible();
		await openTrigger.click();

		const returnTrigger = workspaceEntry.getByRole('button', {
			name: 'Return workspace to main window',
		});
		await expect(returnTrigger).toBeVisible();
		await returnTrigger.click();

		await expect(
			workspaceEntry.getByRole('button', { name: 'Open workspace popout' }),
		).toBeVisible();
	});

	test('renders popout shell with workspace-scoped navigation', async ({ page }) => {
		await gotoWorkspacePopoutView(page, e2eWorkspaceFixture.workspaceName, 'command-center');
		await expect(page.getByRole('button', { name: 'Workset Hub' })).toHaveCount(0);
		await expect(page.getByRole('button', { name: 'New Workset' })).toHaveCount(0);
		await expect(page.getByRole('button', { name: 'Skill Registry' })).toHaveCount(0);

		await expect(page.getByRole('button', { name: 'Command Center' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Engineering Cockpit' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'PR Orchestration' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Return to main window' })).toBeVisible();
		await expect(page.locator('.panel-title')).toHaveText(e2eWorkspaceFixture.workspaceName);
	});
});
