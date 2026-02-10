import { existsSync, mkdirSync } from 'node:fs';
import path from 'node:path';
import { expect, type Page } from '@playwright/test';

export type SnapshotRepo = {
	name: string;
	dirty: boolean;
	currentBranch?: string;
	defaultBranch?: string;
	ahead?: number;
	behind?: number;
	files?: Array<{ path: string }>;
	trackedPullRequest?: { state: string };
};

export type WorkspaceSnapshot = {
	name: string;
	pinned?: boolean;
	repos: SnapshotRepo[];
};

export type GroupSummary = {
	name: string;
};

export type AliasSummary = {
	name: string;
};

type SnapshotOptions = {
	includeArchived: boolean;
	includeStatus: boolean;
};

type WorkspacePopoutState = {
	workspaceId: string;
	windowName: string;
	open: boolean;
};

type WorkspaceCreateInput = {
	name: string;
	path: string;
	repos?: string[];
	groups?: string[];
};

type RepoAddInput = {
	workspaceId: string;
	source: string;
	name?: string;
	repoDir?: string;
};

type WorkspaceRemoveInput = {
	workspaceId: string;
	deleteFiles: boolean;
	force: boolean;
	fetchRemotes: boolean;
};

type AppBindings = {
	ListWorkspaceSnapshots?: (input: SnapshotOptions) => Promise<WorkspaceSnapshot[]>;
	ListAliases?: () => Promise<AliasSummary[]>;
	ListGroups?: () => Promise<GroupSummary[]>;
	CreateWorkspace?: (input: WorkspaceCreateInput) => Promise<unknown>;
	AddRepo?: (input: RepoAddInput) => Promise<unknown>;
	RemoveWorkspace?: (input: WorkspaceRemoveInput) => Promise<unknown>;
	ListWorkspacePopouts?: () => Promise<WorkspacePopoutState[]>;
	CloseWorkspacePopout?: (workspaceId: string) => Promise<void>;
};

export type E2EWorkspaceFixture = {
	workspaceName: string;
	workspacePath: string;
	repoName: string;
	repoSource: string;
};

export const e2eWorkspaceFixture: E2EWorkspaceFixture = {
	workspaceName: process.env.WORKSET_E2E_WORKSPACE_NAME ?? 'e2e-workspace-fixture',
	workspacePath: process.env.WORKSET_E2E_WORKSPACE_PATH ?? '/tmp/workset-e2e-workspaces',
	repoName: process.env.WORKSET_E2E_REPO_NAME ?? 'workset',
	repoSource: process.env.WORKSET_E2E_REPO_SOURCE ?? path.resolve(process.cwd(), '../../..'),
};

export const hasWailsBindings = async (page: Page): Promise<boolean> =>
	page.evaluate(() => {
		const app = (window as { go?: { main?: { App?: unknown } } }).go?.main?.App;
		return !!app;
	});

export const assertWailsBindings = async (page: Page): Promise<void> => {
	const hasBindings = await hasWailsBindings(page);
	if (!hasBindings) {
		throw new Error(
			'Wails bindings are unavailable in this test run. Start tests against `wails3 dev` and ensure runtime bindings are injected.',
		);
	}
};

export const gotoApp = async (page: Page): Promise<void> => {
	await page.goto('/');
	await dismissGitHubAuthModalIfPresent(page);
	await expect(page.locator('.app-shell')).toBeVisible();
};

export const dismissGitHubAuthModalIfPresent = async (page: Page): Promise<void> => {
	const dialog = page.getByRole('dialog', { name: 'Connect GitHub' });
	if ((await dialog.count()) === 0) return;
	if (!(await dialog.first().isVisible())) return;

	const dismissButton = dialog.getByRole('button', { name: /Not now|Cancel/i }).first();
	if ((await dismissButton.count()) > 0) {
		await dismissButton.click();
	}
};

export const listWorkspaceSnapshots = async (
	page: Page,
	options: SnapshotOptions = { includeArchived: false, includeStatus: true },
): Promise<WorkspaceSnapshot[]> =>
	page.evaluate(
		async ({ opts }) => {
			const app = (window as { go?: { main?: { App?: AppBindings } } }).go?.main?.App;
			if (!app?.ListWorkspaceSnapshots) return [];
			return app.ListWorkspaceSnapshots(opts);
		},
		{ opts: options },
	);

export const listAliases = async (page: Page): Promise<AliasSummary[]> =>
	page.evaluate(async () => {
		const app = (window as { go?: { main?: { App?: AppBindings } } }).go?.main?.App;
		if (!app?.ListAliases) return [];
		return app.ListAliases();
	});

export const listGroups = async (page: Page): Promise<GroupSummary[]> =>
	page.evaluate(async () => {
		const app = (window as { go?: { main?: { App?: AppBindings } } }).go?.main?.App;
		if (!app?.ListGroups) return [];
		return app.ListGroups();
	});

export const closeWorkspacePopoutIfOpen = async (
	page: Page,
	workspaceId: string,
): Promise<void> => {
	await page.evaluate(
		async ({ id }) => {
			const app = (window as { go?: { main?: { App?: AppBindings } } }).go?.main?.App;
			if (!app?.ListWorkspacePopouts || !app?.CloseWorkspacePopout) return;
			const popouts = await app.ListWorkspacePopouts();
			const isOpen = popouts.some((entry) => entry.workspaceId === id && entry.open);
			if (!isOpen) return;
			await app.CloseWorkspacePopout(id);
		},
		{ id: workspaceId },
	);
};

export const waitForWorkspaceSnapshot = async (
	page: Page,
	workspaceName: string,
	minRepos = 0,
	timeoutMs = 30_000,
): Promise<WorkspaceSnapshot> => {
	const deadline = Date.now() + timeoutMs;
	while (Date.now() < deadline) {
		const snapshots = await listWorkspaceSnapshots(page, {
			includeArchived: true,
			includeStatus: true,
		});
		const snapshot = snapshots.find((workspace) => workspace.name === workspaceName);
		if (snapshot && snapshot.repos.length >= minRepos) {
			return snapshot;
		}
		await page.waitForTimeout(250);
	}
	throw new Error(
		`Timed out waiting for workspace "${workspaceName}" with at least ${minRepos} repo(s).`,
	);
};

const removeWorkspaceByName = async (page: Page, workspaceName: string): Promise<void> => {
	const snapshots = await listWorkspaceSnapshots(page, {
		includeArchived: true,
		includeStatus: false,
	});
	const matches = snapshots.filter((workspace) => workspace.name === workspaceName);
	for (const workspace of matches) {
		await closeWorkspacePopoutIfOpen(page, workspace.name);
		await page.evaluate(
			async ({ workspaceId }) => {
				const app = (window as { go?: { main?: { App?: AppBindings } } }).go?.main?.App;
				if (!app?.RemoveWorkspace) {
					throw new Error('RemoveWorkspace binding unavailable');
				}
				await app.RemoveWorkspace({
					workspaceId,
					deleteFiles: false,
					force: true,
					fetchRemotes: false,
				});
			},
			{ workspaceId: workspace.name },
		);
	}
};

export const ensureE2EWorkspaceFixture = async (
	page: Page,
	fixture: E2EWorkspaceFixture = e2eWorkspaceFixture,
): Promise<WorkspaceSnapshot> => {
	await assertWailsBindings(page);
	if (!existsSync(fixture.repoSource)) {
		throw new Error(
			`Fixture repo source does not exist: ${fixture.repoSource}. Set WORKSET_E2E_REPO_SOURCE to a valid local git repo path.`,
		);
	}
	mkdirSync(fixture.workspacePath, { recursive: true });

	await removeWorkspaceByName(page, fixture.workspaceName);
	await page.evaluate(
		async ({ workspaceName, workspacePath }) => {
			const app = (window as { go?: { main?: { App?: AppBindings } } }).go?.main?.App;
			if (!app?.CreateWorkspace) {
				throw new Error('CreateWorkspace binding unavailable');
			}
			await app.CreateWorkspace({
				name: workspaceName,
				path: workspacePath,
				repos: [],
				groups: [],
			});
		},
		{ workspaceName: fixture.workspaceName, workspacePath: fixture.workspacePath },
	);

	await page.evaluate(
		async ({ workspaceId, source, repoName }) => {
			const app = (window as { go?: { main?: { App?: AppBindings } } }).go?.main?.App;
			if (!app?.AddRepo) {
				throw new Error('AddRepo binding unavailable');
			}
			await app.AddRepo({
				workspaceId,
				source,
				name: repoName,
				repoDir: '',
			});
		},
		{
			workspaceId: fixture.workspaceName,
			source: fixture.repoSource,
			repoName: fixture.repoName,
		},
	);

	return waitForWorkspaceSnapshot(page, fixture.workspaceName, 1, 60_000);
};

export const openMainRailView = async (page: Page, label: string): Promise<void> => {
	await page.getByRole('button', { name: label }).click();
};

export const openWorksetHub = async (page: Page): Promise<void> => {
	await openMainRailView(page, 'Workset Hub');
	await expect(page.getByRole('heading', { name: 'Worksets' })).toBeVisible();
};

export const selectWorkspaceFromHubByName = async (
	page: Page,
	workspaceName: string,
): Promise<string> => {
	await openWorksetHub(page);
	const workspaceEntry = page
		.locator('.workset-card, .list-row', { hasText: workspaceName })
		.first();
	await expect(workspaceEntry).toBeVisible();
	await workspaceEntry.click();
	await expect(page.getByRole('heading', { name: 'Command Center' })).toBeVisible();
	await expect(page.locator('.panel-title')).toHaveText(workspaceName);
	return workspaceName;
};

export const selectFirstWorkspaceFromHub = async (page: Page): Promise<string | null> => {
	await openWorksetHub(page);
	const emptyState = page.getByText('Create your first workspace');
	if ((await emptyState.count()) > 0 && (await emptyState.first().isVisible())) {
		return null;
	}

	const cards = page.locator('.workset-card, .list-row');
	await expect(cards.first()).toBeVisible();
	await cards.first().click();
	await expect(page.getByRole('heading', { name: 'Command Center' })).toBeVisible();
	return (await page.locator('.panel-title').first().innerText()).trim();
};

export const toggleWorkspacePopoutFromHub = async (
	page: Page,
	workspaceName: string,
): Promise<void> => {
	await openWorksetHub(page);
	const workspaceEntry = page
		.locator('.workset-card, .list-row', { hasText: workspaceName })
		.first();
	await expect(workspaceEntry).toBeVisible();
	const trigger = workspaceEntry.getByRole('button', {
		name: /Open workspace popout|Return workspace to main window/i,
	});
	await expect(trigger).toBeVisible();
	await trigger.click();
};

export const gotoWorkspacePopoutView = async (
	page: Page,
	workspaceId: string,
	view: 'command-center' | 'terminal-cockpit' | 'pr-orchestration' = 'command-center',
): Promise<void> => {
	const search = new URLSearchParams({
		popout: '1',
		workspace: workspaceId,
		view,
	});
	await page.goto(`/?${search.toString()}`);
	await expect(page.locator('.app-shell.popout')).toBeVisible();
	await dismissGitHubAuthModalIfPresent(page);
};

export const readStatCard = async (page: Page, label: string): Promise<number> => {
	const card = page.locator('.stat-card').filter({ hasText: label }).first();
	return Number.parseInt((await card.locator('strong').innerText()).trim(), 10);
};

export const readPillCount = async (page: Page, label: string): Promise<number> => {
	const pill = page.locator('.stat-pill').filter({ hasText: label }).first();
	return Number.parseInt((await pill.locator('strong').innerText()).trim(), 10);
};

export const commandPaletteShortcut = (): string =>
	process.platform === 'darwin' ? 'Meta+K' : 'Control+K';
