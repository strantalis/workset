import { describe, test, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import Button from './Button.svelte';
import { asSnippet } from '../../test-utils/snippet';

describe('Button', () => {
	afterEach(() => {
		cleanup();
	});

	test('renders with default props', () => {
		const { getByRole } = render(Button, {
			props: {
				children: asSnippet('Click me'),
			},
		});
		const button = getByRole('button');
		expect(button).toBeInTheDocument();
	});

	test('renders different variants', () => {
		const { container } = render(Button, {
			props: {
				variant: 'primary',
				children: asSnippet('Primary Button'),
			},
		});
		const button = container.querySelector('button');
		expect(button).toHaveClass('primary');
	});

	test('handles click events', async () => {
		const handleClick = vi.fn();
		const { getByRole } = render(Button, {
			props: {
				onclick: handleClick,
				children: asSnippet('Click me'),
			},
		});
		const button = getByRole('button');
		await fireEvent.click(button);
		expect(handleClick).toHaveBeenCalledTimes(1);
	});

	test('is disabled when disabled prop is true', () => {
		const handleClick = vi.fn();
		const { getByRole } = render(Button, {
			props: {
				disabled: true,
				onclick: handleClick,
				children: asSnippet('Disabled Button'),
			},
		});
		const button = getByRole('button');
		expect(button).toBeDisabled();
	});

	test('renders different sizes', () => {
		const { container } = render(Button, {
			props: {
				size: 'sm',
				children: asSnippet('Small Button'),
			},
		});
		const button = container.querySelector('button');
		expect(button).toHaveClass('sm');
	});

	test('renders as submit type', () => {
		const { getByRole } = render(Button, {
			props: {
				type: 'submit',
				children: asSnippet('Submit'),
			},
		});
		const button = getByRole('button');
		expect(button).toHaveAttribute('type', 'submit');
	});
});
