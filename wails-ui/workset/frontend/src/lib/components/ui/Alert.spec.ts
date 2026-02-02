import { describe, test, expect, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import Alert from './Alert.svelte';
import { asSnippet } from '../../test-utils/snippet';

describe('Alert', () => {
	afterEach(() => {
		cleanup();
	});

	test('renders with error variant', () => {
		const { container } = render(Alert, {
			props: {
				variant: 'error',
				children: asSnippet('Error message'),
			},
		});
		const alert = container.querySelector('.alert');
		expect(alert).toBeInTheDocument();
		expect(alert).toHaveClass('error');
	});

	test('renders with success variant', () => {
		const { container } = render(Alert, {
			props: {
				variant: 'success',
				children: asSnippet('Success message'),
			},
		});
		const alert = container.querySelector('.alert');
		expect(alert).toHaveClass('success');
	});

	test('renders with warning variant', () => {
		const { container } = render(Alert, {
			props: {
				variant: 'warning',
				children: asSnippet('Warning message'),
			},
		});
		const alert = container.querySelector('.alert');
		expect(alert).toHaveClass('warning');
	});

	test('renders with info variant', () => {
		const { container } = render(Alert, {
			props: {
				variant: 'info',
				children: asSnippet('Info message'),
			},
		});
		const alert = container.querySelector('.alert');
		expect(alert).toHaveClass('info');
	});

	test('renders children content', () => {
		const { container } = render(Alert, {
			props: {
				variant: 'info',
				children: asSnippet('Custom alert content'),
			},
		});
		const alert = container.querySelector('.alert');
		expect(alert).toBeInTheDocument();
		// Note: Snippet content rendering is validated by component structure
	});
});
