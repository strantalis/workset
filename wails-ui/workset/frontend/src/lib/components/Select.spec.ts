import { describe, test, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import Select from './ui/Select.svelte';

describe('Select', () => {
	const options = [
		{ value: 'option1', label: 'Option 1' },
		{ value: 'option2', label: 'Option 2' },
		{ value: 'option3', label: 'Option 3' },
	];

	afterEach(() => {
		cleanup();
	});

	test('renders with placeholder when no value selected', () => {
		const { getByText } = render(Select, {
			props: {
				value: '',
				options,
				placeholder: 'Choose an option',
			},
		});
		expect(getByText('Choose an option')).toBeInTheDocument();
	});

	test('renders with selected value label', () => {
		const { getByText } = render(Select, {
			props: {
				value: 'option2',
				options,
			},
		});
		expect(getByText('Option 2')).toBeInTheDocument();
	});

	test('opens dropdown when clicked', async () => {
		const { container } = render(Select, {
			props: {
				value: '',
				options,
			},
		});
		const trigger = container.querySelector('.select-trigger');
		await fireEvent.click(trigger!);
		// Dropdown menu should be visible
		const menu = document.querySelector('.select-menu');
		expect(menu).toBeInTheDocument();
	});

	test('is disabled when disabled prop is true', () => {
		const { container } = render(Select, {
			props: {
				value: '',
				options,
				disabled: true,
			},
		});
		const trigger = container.querySelector('.select-trigger');
		expect(trigger).toBeDisabled();
	});

	test('calls onchange when option selected', async () => {
		const handleChange = vi.fn();
		const { container, getByText } = render(Select, {
			props: {
				value: '',
				options,
				onchange: handleChange,
			},
		});

		// Open dropdown
		const trigger = container.querySelector('.select-trigger');
		await fireEvent.click(trigger!);

		// Click an option
		const option = getByText('Option 2');
		await fireEvent.click(option);

		expect(handleChange).toHaveBeenCalledWith('option2');
	});

	test('renders with id attribute', () => {
		const { container } = render(Select, {
			props: {
				id: 'test-select',
				value: '',
				options,
			},
		});
		const trigger = container.querySelector('#test-select');
		expect(trigger).toBeInTheDocument();
	});

	test('has correct ARIA attributes', () => {
		const { container } = render(Select, {
			props: {
				value: '',
				options,
			},
		});
		const trigger = container.querySelector('.select-trigger');
		expect(trigger).toHaveAttribute('aria-haspopup', 'listbox');
		expect(trigger).toHaveAttribute('aria-expanded', 'false');
	});
});
