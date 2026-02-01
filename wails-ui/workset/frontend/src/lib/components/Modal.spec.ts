import { describe, test, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import Modal from './Modal.svelte';

describe('Modal', () => {
	afterEach(() => {
		cleanup();
	});

	test('renders with title', () => {
		const { getByText } = render(Modal, {
			props: {
				title: 'Test Modal',
				children: () => 'Modal content',
			},
		});
		expect(getByText('Test Modal')).toBeInTheDocument();
	});

	test('renders with subtitle when provided', () => {
		const { getByText } = render(Modal, {
			props: {
				title: 'Test Modal',
				subtitle: 'Modal subtitle',
				children: () => 'Modal content',
			},
		});
		expect(getByText('Modal subtitle')).toBeInTheDocument();
	});

	test('renders different sizes', () => {
		const sizes = ['sm', 'md', 'lg', 'xl', 'full'] as const;
		sizes.forEach((size) => {
			const { container } = render(Modal, {
				props: {
					title: 'Test',
					size,
					children: () => 'Content',
				},
			});
			const modal = container.querySelector('.modal');
			expect(modal).toBeInTheDocument();
			cleanup();
		});
	});

	test('renders close button when onClose provided', () => {
		const handleClose = vi.fn();
		const { getByRole } = render(Modal, {
			props: {
				title: 'Test Modal',
				onClose: handleClose,
				children: () => 'Content',
			},
		});
		const closeButton = getByRole('button', { name: /close/i });
		expect(closeButton).toBeInTheDocument();
	});

	test('calls onClose when close button clicked', async () => {
		const handleClose = vi.fn();
		const { getByRole } = render(Modal, {
			props: {
				title: 'Test Modal',
				onClose: handleClose,
				children: () => 'Content',
			},
		});
		const closeButton = getByRole('button', { name: /close/i });
		await fireEvent.click(closeButton);
		expect(handleClose).toHaveBeenCalledTimes(1);
	});

	test('does not render close button when onClose not provided', () => {
		const { container } = render(Modal, {
			props: {
				title: 'Test Modal',
				children: () => 'Content',
			},
		});
		const buttons = container.querySelectorAll('button');
		expect(buttons.length).toBe(0);
	});

	test('disables close button when disableClose is true', () => {
		const handleClose = vi.fn();
		const { getByRole } = render(Modal, {
			props: {
				title: 'Test Modal',
				onClose: handleClose,
				disableClose: true,
				children: () => 'Content',
			},
		});
		const closeButton = getByRole('button', { name: /close/i });
		expect(closeButton).toBeDisabled();
	});

	test('renders left header alignment', () => {
		const handleClose = vi.fn();
		const { container } = render(Modal, {
			props: {
				title: 'Test Modal',
				headerAlign: 'left',
				onClose: handleClose,
				children: () => 'Content',
			},
		});
		const header = container.querySelector('.modal-header');
		expect(header).toHaveClass('left');
	});

	test('renders footer when provided', () => {
		const { container } = render(Modal, {
			props: {
				title: 'Test Modal',
				children: () => 'Content',
				footer: () => 'Footer content',
			},
		});
		const footer = container.querySelector('.modal-footer');
		expect(footer).toBeInTheDocument();
	});
});
