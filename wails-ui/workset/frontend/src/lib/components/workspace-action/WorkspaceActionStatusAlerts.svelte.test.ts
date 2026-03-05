/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { mount, unmount } from 'svelte';
import WorkspaceActionStatusAlerts from './WorkspaceActionStatusAlerts.svelte';

describe('WorkspaceActionStatusAlerts', () => {
	let container: HTMLDivElement;

	beforeEach(() => {
		container = document.createElement('div');
		document.body.appendChild(container);
	});

	afterEach(() => {
		container.remove();
	});

	test('renders hook activity by default', () => {
		const component = mount(WorkspaceActionStatusAlerts, {
			target: container,
			props: {
				error: null,
				success: 'Created alpha/thread-1.',
				warnings: [],
				hookRuns: [{ event: 'workspace.create', repo: 'repo-a', id: 'hook-1', status: 'running' }],
				pendingHooks: [],
				onRunPendingHook: vi.fn(),
				onTrustPendingHook: vi.fn(),
			},
		});

		expect(container).toHaveTextContent('Created alpha/thread-1.');
		expect(container).toHaveTextContent('Hook runs');
		expect(container).toHaveTextContent('hook-1');

		unmount(component);
	});

	test('supports rendering hooks-only sections', () => {
		const component = mount(WorkspaceActionStatusAlerts, {
			target: container,
			props: {
				error: null,
				success: 'Created alpha/thread-1.',
				warnings: ['Heads up'],
				hookRuns: [{ event: 'workspace.create', repo: 'repo-a', id: 'hook-1', status: 'ok' }],
				pendingHooks: [],
				showMessages: false,
				showHooks: true,
				onRunPendingHook: vi.fn(),
				onTrustPendingHook: vi.fn(),
			},
		});

		expect(container).not.toHaveTextContent('Created alpha/thread-1.');
		expect(container).not.toHaveTextContent('Heads up');
		expect(container).toHaveTextContent('Hook runs');
		expect(container).toHaveTextContent('hook-1');

		unmount(component);
	});
});
