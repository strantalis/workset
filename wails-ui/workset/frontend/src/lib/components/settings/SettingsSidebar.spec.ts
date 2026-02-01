import { describe, test, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import SettingsSidebar from './SettingsSidebar.svelte';

describe('SettingsSidebar', () => {
	afterEach(() => {
		cleanup();
	});

	test('renders all section groups including INFO', () => {
		const { getByText } = render(SettingsSidebar, {
			props: {
				activeSection: 'workspace',
				onSelectSection: () => {},
				aliasCount: 0,
				groupCount: 0,
			},
		});

		// Check all group titles are present
		expect(getByText('GENERAL')).toBeInTheDocument();
		expect(getByText('INTEGRATIONS')).toBeInTheDocument();
		expect(getByText('LIBRARY')).toBeInTheDocument();
		expect(getByText('INFO')).toBeInTheDocument();
	});

	test('renders About item in INFO section', () => {
		const { getByText } = render(SettingsSidebar, {
			props: {
				activeSection: 'workspace',
				onSelectSection: () => {},
				aliasCount: 0,
				groupCount: 0,
			},
		});

		expect(getByText('About')).toBeInTheDocument();
	});

	test('renders all navigation items', () => {
		const { getByText } = render(SettingsSidebar, {
			props: {
				activeSection: 'workspace',
				onSelectSection: () => {},
				aliasCount: 0,
				groupCount: 0,
			},
		});

		// GENERAL items
		expect(getByText('Workspace')).toBeInTheDocument();
		expect(getByText('Agent')).toBeInTheDocument();
		expect(getByText('Terminal')).toBeInTheDocument();

		// INTEGRATIONS items
		expect(getByText('GitHub')).toBeInTheDocument();

		// LIBRARY items
		expect(getByText('Aliases')).toBeInTheDocument();
		expect(getByText('Groups')).toBeInTheDocument();

		// INFO items
		expect(getByText('About')).toBeInTheDocument();
	});

	test('displays badge counts for aliases and groups', () => {
		const { getByText } = render(SettingsSidebar, {
			props: {
				activeSection: 'workspace',
				onSelectSection: () => {},
				aliasCount: 5,
				groupCount: 3,
			},
		});

		expect(getByText('5')).toBeInTheDocument();
		expect(getByText('3')).toBeInTheDocument();
	});

	test('marks active section', () => {
		const { container } = render(SettingsSidebar, {
			props: {
				activeSection: 'about',
				onSelectSection: () => {},
				aliasCount: 0,
				groupCount: 0,
			},
		});

		const aboutButton = container.querySelector('button.item.active');
		expect(aboutButton).toBeInTheDocument();
		expect(aboutButton?.textContent).toContain('About');
	});

	test('calls onSelectSection when item is clicked', async () => {
		const handleSelect = vi.fn();
		const { getByText } = render(SettingsSidebar, {
			props: {
				activeSection: 'workspace',
				onSelectSection: handleSelect,
				aliasCount: 0,
				groupCount: 0,
			},
		});

		const aboutButton = getByText('About');
		await fireEvent.click(aboutButton);

		expect(handleSelect).toHaveBeenCalledWith('about');
	});
});
