/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { mount, unmount } from 'svelte';
import OnboardingView from './OnboardingView.svelte';

vi.mock('../../services/workspaceActionModalActions', () => ({
	runWorkspaceActionPendingHook: vi.fn(
		async ({
			pending,
			pendingHooks,
			hookRuns,
			setPendingHooks,
			setHookRuns,
		}: {
			pending: { repo: string; event: string; hooks: string[] };
			pendingHooks: Array<{ repo: string; event: string; hooks: string[] }>;
			hookRuns: Array<{ repo: string; event: string; id: string; status: string }>;
			setPendingHooks: (next: Array<{ repo: string; event: string; hooks: string[] }>) => void;
			setHookRuns: (
				next: Array<{ repo: string; event: string; id: string; status: string }>,
			) => void;
		}) => {
			setHookRuns([
				...hookRuns,
				{ repo: pending.repo, event: pending.event, id: 'mock-hook', status: 'ok' },
			]);
			setPendingHooks(pendingHooks.filter((entry) => entry.repo !== pending.repo));
		},
	),
	trustWorkspaceActionPendingHook: vi.fn(
		async ({
			pending,
			pendingHooks,
			setPendingHooks,
		}: {
			pending: { repo: string };
			pendingHooks: Array<{
				repo: string;
				event: string;
				hooks: string[];
				trusted?: boolean;
				trusting?: boolean;
			}>;
			setPendingHooks: (
				next: Array<{
					repo: string;
					event: string;
					hooks: string[];
					trusted?: boolean;
					trusting?: boolean;
				}>,
			) => void;
		}) => {
			setPendingHooks(
				pendingHooks.map((entry) =>
					entry.repo === pending.repo
						? { ...entry, trusting: false, trusted: true, runError: undefined }
						: entry,
				),
			);
		},
	),
}));

const defaultTemplate = {
	id: 'group-template',
	name: 'API Platform',
	description: 'Template',
	groupName: 'api-group',
	repos: [
		{
			name: 'api-repo',
			remoteUrl: 'git@github.com:example/api-repo.git',
			hooks: [],
			aliasName: 'api-repo',
			sourceType: 'alias' as const,
		},
	],
};

const defaultRegistryRepo = {
	id: 'repo-1',
	name: 'api-repo',
	aliasName: 'api-repo',
	remoteUrl: 'git@github.com:example/api-repo.git',
	defaultBranch: 'main',
	language: 'TypeScript',
	tags: ['remote'],
};

const mountView = (props: Record<string, unknown> = {}) => {
	return mount(OnboardingView, {
		target: document.body,
		props: {
			defaultWorkspaceName: 'workspace-alpha',
			templates: [defaultTemplate],
			repoRegistry: [defaultRegistryRepo],
			...props,
		},
	});
};

const getButton = (label: string): HTMLButtonElement => {
	const button = Array.from(document.querySelectorAll('button')).find((candidate) =>
		candidate.textContent?.includes(label),
	);
	if (!button) {
		throw new Error(`Button not found: ${label}`);
	}
	return button as HTMLButtonElement;
};

const clickButton = async (label: string): Promise<void> => {
	const button = getButton(label);
	button.click();
	await Promise.resolve();
};

describe('OnboardingView', () => {
	let component: ReturnType<typeof mountView> | null = null;
	let previousAnimate: typeof Element.prototype.animate | undefined;

	beforeEach(() => {
		document.body.innerHTML = '';
		previousAnimate = Element.prototype.animate;
		Object.defineProperty(Element.prototype, 'animate', {
			configurable: true,
			writable: true,
			value: vi.fn(() => ({ onfinish: null, cancel: vi.fn(), play: vi.fn() })),
		});
	});

	afterEach(() => {
		if (component) {
			unmount(component);
			component = null;
		}
		document.body.innerHTML = '';
		if (previousAnimate) {
			Object.defineProperty(Element.prototype, 'animate', {
				configurable: true,
				writable: true,
				value: previousAnimate,
			});
		} else {
			delete (Element.prototype as { animate?: unknown }).animate;
		}
	});

	test('previews hooks before initialize and renders discovered hook IDs in review', async () => {
		const onPreviewHooks = vi.fn(async () => ['bootstrap', 'build']);
		component = mountView({ onPreviewHooks });
		await Promise.resolve();

		await clickButton('Continue');
		await clickButton('From Template');
		await clickButton('Next Step');
		await Promise.resolve();
		await Promise.resolve();

		expect(onPreviewHooks).toHaveBeenCalledTimes(1);
		expect(onPreviewHooks).toHaveBeenCalledWith('api-repo');
		expect(document.body).toHaveTextContent('Discovered lifecycle hooks');
		expect(document.body).toHaveTextContent('bootstrap');
		expect(document.body).toHaveTextContent('build');
	});

	test('blocks duplicate workspace names before moving to next step', async () => {
		component = mountView({
			existingWorkspaceNames: ['workspace-alpha'],
		});
		await Promise.resolve();

		const continueButton = getButton('Continue');
		expect(continueButton.disabled).toBe(true);
		expect(document.body).toHaveTextContent('A workset named "workspace-alpha" already exists.');

		const nameInput = document.querySelector('input[type="text"]');
		if (!(nameInput instanceof HTMLInputElement)) {
			throw new Error('Workspace name input not found');
		}
		nameInput.value = 'workspace-beta';
		nameInput.dispatchEvent(new Event('input', { bubbles: true }));
		await Promise.resolve();

		expect(getButton('Continue').disabled).toBe(false);
		expect(document.body).not.toHaveTextContent(
			'A workset named "workspace-alpha" already exists.',
		);
	});

	test('requires pending hooks to be trusted before open and then calls onComplete', async () => {
		const onStart = vi.fn().mockResolvedValue({
			workspaceName: 'workspace-alpha',
			warnings: [],
			pendingHooks: [{ event: 'workspace.create', repo: 'api-repo', hooks: ['post-checkout'] }],
			hookRuns: [],
		});
		const onComplete = vi.fn();
		component = mountView({ onStart, onComplete });
		await Promise.resolve();

		await clickButton('Continue');
		await clickButton('From Template');
		await clickButton('Next Step');
		await clickButton('Initialize Workset');

		await Promise.resolve();
		await Promise.resolve();

		expect(onStart).toHaveBeenCalledTimes(1);
		expect(document.body).toHaveTextContent('Resolve Hook Trust To Continue');
		expect(document.body).toHaveTextContent('api-repo');

		await clickButton('Trust');
		await Promise.resolve();

		expect(document.body).toHaveTextContent('Open Workset');

		await clickButton('Open Workset');
		await Promise.resolve();

		expect(onComplete).toHaveBeenCalledTimes(1);
		expect(onComplete).toHaveBeenCalledWith('workspace-alpha');
	});
});
