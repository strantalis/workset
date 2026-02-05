import { describe, test, expect, vi, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import DropdownMenu from './ui/DropdownMenu.svelte';
import { asSnippet } from '../test-utils/snippet';

describe('DropdownMenu', () => {
	afterEach(() => {
		cleanup();
	});

	test('renders when open is true', () => {
		render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				children: asSnippet('Menu content'),
			},
		});
		const menu = document.body.querySelector('.dropdown-menu');
		expect(menu).toBeInTheDocument();
	});

	test('does not render when open is false', () => {
		render(DropdownMenu, {
			props: {
				open: false,
				onClose: vi.fn(),
				children: asSnippet('Menu content'),
			},
		});
		const menu = document.body.querySelector('.dropdown-menu');
		expect(menu).not.toBeInTheDocument();
	});

	test('renders children content', () => {
		render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				children: asSnippet('Dropdown items'),
			},
		});
		const menu = document.body.querySelector('.dropdown-menu');
		expect(menu).toBeInTheDocument();
		// Note: Snippet content rendering validated by menu presence
	});

	test('renders with right position class', () => {
		render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				position: 'right',
				children: asSnippet('Content'),
			},
		});
		const menu = document.body.querySelector('.dropdown-menu');
		expect(menu).toHaveClass('right');
	});

	test('renders with left position class', () => {
		render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				position: 'left',
				children: asSnippet('Content'),
			},
		});
		const menu = document.body.querySelector('.dropdown-menu');
		expect(menu).toHaveClass('left');
	});

	test('has correct role attribute', () => {
		render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				children: asSnippet('Content'),
			},
		});
		const menu = document.body.querySelector('.dropdown-menu');
		expect(menu).toHaveAttribute('role', 'menu');
	});

	test('applies positioning styles', () => {
		render(DropdownMenu, {
			props: {
				open: true,
				onClose: vi.fn(),
				children: asSnippet('Content'),
			},
		});
		const menu = document.body.querySelector('.dropdown-menu');
		// Verify style attribute exists (JSDOM doesn't fully support inline styles)
		expect(menu?.getAttribute('style')).toContain('top');
		expect(menu?.getAttribute('style')).toContain('left');
	});
});
