import { describe, test, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import IconButton from './IconButton.svelte';

describe('IconButton', () => {
	afterEach(() => {
		cleanup();
	});

	test('renders with default size', () => {
		const { container } = render(IconButton, {
			props: {
				label: 'Settings',
				children: () => 'Icon',
			},
		});
		const button = container.querySelector('button');
		expect(button).toBeInTheDocument();
		expect(button).toHaveAttribute('aria-label', 'Settings');
		expect(button).toHaveClass('md');
	});

	test('renders different sizes', () => {
		const sizes = ['sm', 'md', 'lg'] as const;
		sizes.forEach((size) => {
			const { container } = render(IconButton, {
				props: {
					size,
					label: `${size} button`,
					children: () => 'Icon',
				},
			});
			const button = container.querySelector('button');
			expect(button).toHaveClass(size);
			cleanup();
		});
	});

	test('handles click events', async () => {
		const handleClick = vi.fn();
		const { container } = render(IconButton, {
			props: {
				label: 'Click me',
				onclick: handleClick,
				children: () => 'Icon',
			},
		});
		const button = container.querySelector('button');
		await fireEvent.click(button!);
		expect(handleClick).toHaveBeenCalledTimes(1);
	});

	test('is disabled when disabled prop is true', () => {
		const handleClick = vi.fn();
		const { container } = render(IconButton, {
			props: {
				label: 'Disabled',
				disabled: true,
				onclick: handleClick,
				children: () => 'Icon',
			},
		});
		const button = container.querySelector('button');
		expect(button).toBeDisabled();
	});

	test('has correct type attribute', () => {
		const { container } = render(IconButton, {
			props: {
				label: 'Action',
				children: () => 'Icon',
			},
		});
		const button = container.querySelector('button');
		expect(button).toHaveAttribute('type', 'button');
	});
});
