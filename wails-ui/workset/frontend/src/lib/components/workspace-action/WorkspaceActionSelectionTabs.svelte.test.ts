/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { mount, unmount } from 'svelte';
import WorkspaceActionSelectionTabs from './WorkspaceActionSelectionTabs.svelte';

describe('WorkspaceActionSelectionTabs', () => {
	let container: HTMLDivElement;

	beforeEach(() => {
		container = document.createElement('div');
		document.body.appendChild(container);
	});

	afterEach(() => {
		container.remove();
	});

	test('renders available tabs and calls onTabChange for clicks', () => {
		const onTabChange = vi.fn();
		const component = mount(WorkspaceActionSelectionTabs, {
			target: container,
			props: {
				activeTab: 'repos',
				aliasCount: 3,
				groupCount: 2,
				onTabChange,
			},
		});

		expect(container).toHaveTextContent('Direct');
		expect(container).toHaveTextContent('Repos (3)');
		expect(container).toHaveTextContent('Groups (2)');
		const reposButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Repos (3)',
		);
		expect(reposButton).toHaveClass('active');

		const directButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Direct',
		);
		const groupsButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Groups (2)',
		);
		directButton?.click();
		groupsButton?.click();

		expect(onTabChange).toHaveBeenNthCalledWith(1, 'direct');
		expect(onTabChange).toHaveBeenNthCalledWith(2, 'groups');

		unmount(component);
	});

	test('does not render tab bar when no alias/group tabs are available', () => {
		const component = mount(WorkspaceActionSelectionTabs, {
			target: container,
			props: {
				activeTab: 'direct',
				aliasCount: 0,
				groupCount: 0,
				onTabChange: vi.fn(),
			},
		});

		expect(container.querySelector('.tab-bar')).toBeNull();
		expect(container.querySelectorAll('button')).toHaveLength(0);

		unmount(component);
	});
});
