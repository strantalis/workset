/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, test } from 'vitest';
import { mount, unmount } from 'svelte';
import RemovalOverlay from './RemovalOverlay.svelte';

describe('RemovalOverlay', () => {
	let container: HTMLDivElement;

	beforeEach(() => {
		container = document.createElement('div');
		document.body.appendChild(container);
	});

	afterEach(() => {
		container.remove();
	});

	test('does not render when not removing', () => {
		const component = mount(RemovalOverlay, {
			target: container,
			props: {
				removing: false,
				removalSuccess: false,
				removingText: 'Removing workspace…',
			},
		});

		expect(container.querySelector('.removal-overlay')).not.toBeInTheDocument();

		unmount(component);
	});

	test('renders loading and success states', () => {
		const loadingComponent = mount(RemovalOverlay, {
			target: container,
			props: {
				removing: true,
				removalSuccess: false,
				removingText: 'Removing repo…',
			},
		});

		expect(container.querySelector('.removal-overlay')).toBeInTheDocument();
		expect(container.querySelector('.spinner')).toBeInTheDocument();
		expect(container).toHaveTextContent('Removing repo…');

		unmount(loadingComponent);
		container.innerHTML = '';

		const successComponent = mount(RemovalOverlay, {
			target: container,
			props: {
				removing: true,
				removalSuccess: true,
				removingText: 'Removing repo…',
			},
		});

		expect(container.querySelector('.success-icon')).toBeInTheDocument();
		expect(container).toHaveTextContent('Removed successfully');

		unmount(successComponent);
	});
});
