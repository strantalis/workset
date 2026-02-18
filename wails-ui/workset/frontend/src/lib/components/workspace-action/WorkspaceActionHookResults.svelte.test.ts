/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { mount, unmount } from 'svelte';
import WorkspaceActionHookResults from './WorkspaceActionHookResults.svelte';

describe('WorkspaceActionHookResults', () => {
	let container: HTMLDivElement;

	beforeEach(() => {
		container = document.createElement('div');
		document.body.appendChild(container);
	});

	afterEach(() => {
		container.remove();
	});

	test('renders hook summary and triggers callbacks', () => {
		const onRunPendingHook = vi.fn();
		const onTrustPendingHook = vi.fn();
		const onDone = vi.fn();
		const pending = {
			event: 'workspace.create',
			repo: 'repo-a',
			hooks: ['post-checkout', 'post-merge'],
			runError: 'Needs trust first',
		};

		const component = mount(WorkspaceActionHookResults, {
			target: container,
			props: {
				success: 'Created alpha.',
				warnings: ['warning one'],
				hookRuns: [
					{
						event: 'workspace.create',
						repo: 'repo-a',
						id: 'hook-1',
						status: 'ok',
						log_path: '/tmp/hook.log',
					},
				],
				pendingHooks: [pending],
				onRunPendingHook,
				onTrustPendingHook,
				onDone,
			},
		});

		expect(container).toHaveTextContent('Created alpha.');
		expect(container).toHaveTextContent('warning one');
		expect(container).toHaveTextContent('hook-1');
		expect(container).toHaveTextContent('Needs trust first');

		const runButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Run now',
		);
		const trustButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Trust',
		);
		const doneButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Done',
		);

		runButton?.click();
		trustButton?.click();
		doneButton?.click();

		expect(onRunPendingHook).toHaveBeenCalledTimes(1);
		expect(onRunPendingHook).toHaveBeenCalledWith(pending);
		expect(onTrustPendingHook).toHaveBeenCalledTimes(1);
		expect(onTrustPendingHook).toHaveBeenCalledWith(pending);
		expect(onDone).toHaveBeenCalledTimes(1);

		unmount(component);
	});

	test('disables pending hook actions for trusted repos', () => {
		const component = mount(WorkspaceActionHookResults, {
			target: container,
			props: {
				success: null,
				warnings: [],
				hookRuns: [],
				pendingHooks: [
					{
						event: 'workspace.create',
						repo: 'repo-b',
						hooks: ['post-checkout'],
						trusted: true,
					},
				],
				onRunPendingHook: vi.fn(),
				onTrustPendingHook: vi.fn(),
				onDone: vi.fn(),
			},
		});

		const runButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Run now',
		);
		const trustButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Trusted',
		);

		expect(runButton).toBeDisabled();
		expect(trustButton).toBeDisabled();
		expect(container).toHaveTextContent('trusted');

		unmount(component);
	});
});
