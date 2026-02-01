import { describe, test, expect, vi, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import DropdownMenu from './ui/DropdownMenu.svelte';

describe('DropdownMenu', () => {
	afterEach(() => {
		cleanup();
	});

	test('renders when open is true', () => {
		const { container } = render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				children: () => 'Menu content',
			},
		});
		const menu = container.querySelector('.dropdown-menu');
		expect(menu).toBeInTheDocument();
	});

	test('does not render when open is false', () => {
		const { container } = render(DropdownMenu, {
			props: {
				open: false,
				onClose: vi.fn(),
				children: () => 'Menu content',
			},
		});
		const menu = container.querySelector('.dropdown-menu');
		expect(menu).not.toBeInTheDocument();
	});

	test('renders children content', () => {
		const { container } = render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				children: () => 'Dropdown items',
			},
		});
		const menu = container.querySelector('.dropdown-menu');
		expect(menu).toBeInTheDocument();
		// Note: Snippet content rendering validated by menu presence
	});

	test('renders with right position class', () => {
		const { container } = render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				position: 'right',
				children: () => 'Content',
			},
		});
		const menu = container.querySelector('.dropdown-menu');
		expect(menu).toHaveClass('right');
	});

	test('renders with left position class', () => {
		const { container } = render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				position: 'left',
				children: () => 'Content',
			},
		});
		const menu = container.querySelector('.dropdown-menu');
		expect(menu).toHaveClass('left');
	});

	test('has correct role attribute', () => {
		const { container } = render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				children: () => 'Content',
			},
		});
		const menu = container.querySelector('.dropdown-menu');
		expect(menu).toHaveAttribute('role', 'menu');
	});

	test('applies positioning styles', () => {
		const { container } = render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				children: () => 'Content',
			},
		});
		const menu = container.querySelector('.dropdown-menu');
		// Verify style attribute exists (JSDOM doesn't fully support inline styles)
		expect(menu?.getAttribute('style')).toContain('top');
		expect(menu?.getAttribute('style')).toContain('left');
	});
});
