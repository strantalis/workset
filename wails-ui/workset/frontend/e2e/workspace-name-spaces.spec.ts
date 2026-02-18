import { randomUUID } from 'node:crypto';
import { expect, test } from '@playwright/test';
import {
	assertWailsBindings,
	e2eWorkspaceFixture,
	ensureE2EWorkspaceFixture,
	gotoApp,
	listWorkspaceSnapshots,
	selectWorkspaceFromHubByName,
} from './helpers/app-harness';

test.describe('workspace names with spaces', () => {
	test('creates and selects workspace with spaced name', async ({ page }) => {
		await gotoApp(page);
		await assertWailsBindings(page);

		const fixture = {
			...e2eWorkspaceFixture,
			workspaceName: 'e2e workspace fixture',
			workspacePath: `/tmp/workset-e2e-workspaces-${randomUUID()}`,
		};

		const snapshot = await ensureE2EWorkspaceFixture(page, fixture);
		expect(snapshot.name).toBe(fixture.workspaceName);
		expect(snapshot.repos.length).toBeGreaterThan(0);

		const snapshots = await listWorkspaceSnapshots(page, {
			includeArchived: true,
			includeStatus: true,
		});
		expect(snapshots.some((workspace) => workspace.name === fixture.workspaceName)).toBeTruthy();

		const selectedWorkspaceName = await selectWorkspaceFromHubByName(page, fixture.workspaceName);
		expect(selectedWorkspaceName).toBe(fixture.workspaceName);
		await expect(page.locator('.panel-title')).toHaveText(fixture.workspaceName);
	});
});
