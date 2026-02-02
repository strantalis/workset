/**
 * @vitest-environment jsdom
 */
import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, unmount } from 'svelte';
import Button from './Button.svelte';

describe('Button - Svelte 5 mount API', () => {
	let container: HTMLDivElement;

	beforeEach(() => {
		container = document.createElement('div');
		document.body.appendChild(container);
	});

	afterEach(() => {
		container.remove();
	});

	test('renders button element', () => {
		const component = mount(Button, {
			target: container,
			props: {
				children: () => 'Click me',
			},
		});

		const button = container.querySelector('button');
		expect(button).toBeInTheDocument();

		unmount(component);
	});

	test('renders different variants', () => {
		const component = mount(Button, {
			target: container,
			props: {
				variant: 'primary',
				children: () => 'Primary Button',
			},
		});

		const button = container.querySelector('button');
		expect(button).toHaveClass('primary');

		unmount(component);
	});

	test('handles click events', () => {
		const handleClick = vi.fn();
		const component = mount(Button, {
			target: container,
			props: {
				onclick: handleClick,
				children: () => 'Click me',
			},
		});

		const button = container.querySelector('button');
		button?.click();

		expect(handleClick).toHaveBeenCalledTimes(1);

		unmount(component);
	});

	test('is disabled when disabled prop is true', () => {
		const component = mount(Button, {
			target: container,
			props: {
				disabled: true,
				children: () => 'Disabled Button',
			},
		});

		const button = container.querySelector('button');
		expect(button).toBeDisabled();

		unmount(component);
	});

	test('renders different sizes', () => {
		const component = mount(Button, {
			target: container,
			props: {
				size: 'sm',
				children: () => 'Small Button',
			},
		});

		const button = container.querySelector('button');
		expect(button).toHaveClass('sm');

		unmount(component);
	});

	test('renders as submit type', () => {
		const component = mount(Button, {
			target: container,
			props: {
				type: 'submit',
				children: () => 'Submit',
			},
		});

		const button = container.querySelector('button');
		expect(button).toHaveAttribute('type', 'submit');

		unmount(component);
	});
});
