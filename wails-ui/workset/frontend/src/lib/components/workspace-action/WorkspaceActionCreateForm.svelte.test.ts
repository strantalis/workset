/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { mount, unmount } from 'svelte';
import type { ComponentProps } from 'svelte';
import type { Alias, GroupSummary } from '../../types';
import WorkspaceActionCreateForm from './WorkspaceActionCreateForm.svelte';

const aliases: Alias[] = [{ name: 'repo-alias', path: '/tmp/repos/repo-alias' }];
const groups: GroupSummary[] = [{ name: 'team-group', description: 'Shared repos', repo_count: 2 }];
const groupDetails = new Map<string, string[]>([['team-group', ['repo-a', 'repo-b']]]);

type CreateFormProps = ComponentProps<typeof WorkspaceActionCreateForm>;

const baseProps = (): CreateFormProps => ({
	loading: false,
	activeTab: 'direct',
	aliasItems: aliases,
	groupItems: groups,
	searchQuery: '',
	primaryInput: '/tmp/repos/new-repo',
	directRepos: [{ url: '/tmp/repos/existing-repo', register: true }],
	filteredAliases: aliases,
	filteredGroups: groups,
	selectedAliases: new Set<string>(),
	selectedGroups: new Set<string>(),
	expandedGroups: new Set<string>(),
	groupDetails,
	selectedItems: [
		{ type: 'repo', name: 'existing-repo', url: '/tmp/repos/existing-repo', pending: false },
	],
	totalRepos: 1,
	customizeName: 'workspace-name',
	generatedName: 'generated-name',
	alternatives: ['alt-1', 'alt-2'],
	finalName: 'workspace-name',
	getAliasSource: (alias: Alias) => alias.path || alias.url || '',
	deriveRepoName: (source: string) => source.split('/').filter(Boolean).at(-1) || source,
	isRepoSource: (source: string) => source.includes('/'),
	onTabChange: vi.fn(),
	onPrimaryInput: vi.fn(),
	onSearchQueryInput: vi.fn(),
	onAddDirectRepo: vi.fn(),
	onBrowsePrimary: vi.fn(),
	onToggleDirectRepoRegister: vi.fn(),
	onRemoveDirectRepo: vi.fn(),
	onToggleAlias: vi.fn(),
	onToggleGroup: vi.fn(),
	onToggleGroupExpand: vi.fn(),
	onRemoveAlias: vi.fn(),
	onRemoveGroup: vi.fn(),
	onCustomizeNameInput: vi.fn(),
	onSelectAlternative: vi.fn(),
	onSubmit: vi.fn(),
});

describe('WorkspaceActionCreateForm', () => {
	let container: HTMLDivElement;

	beforeEach(() => {
		container = document.createElement('div');
		document.body.appendChild(container);
	});

	afterEach(() => {
		container.remove();
	});

	test('handles direct tab callbacks for add, browse, register and submit', () => {
		const props = baseProps();
		const component = mount(WorkspaceActionCreateForm, {
			target: container,
			props,
		});

		const sourceInput = container.querySelector(
			'input[placeholder="git@github.com:org/repo.git"]',
		) as HTMLInputElement;
		sourceInput.value = '/tmp/repos/changed-repo';
		sourceInput.dispatchEvent(new Event('input', { bubbles: true }));
		expect(props.onPrimaryInput).toHaveBeenCalledWith('/tmp/repos/changed-repo');

		sourceInput.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
		expect(props.onAddDirectRepo).toHaveBeenCalledTimes(1);

		const addButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Add',
		);
		addButton?.click();
		expect(props.onAddDirectRepo).toHaveBeenCalledTimes(2);

		const browseButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Browse',
		);
		browseButton?.click();
		expect(props.onBrowsePrimary).toHaveBeenCalledTimes(1);

		const registerCheckbox = container.querySelector(
			'.direct-repo-register input[type="checkbox"]',
		) as HTMLInputElement;
		registerCheckbox.click();
		expect(props.onToggleDirectRepoRegister).toHaveBeenCalledWith('/tmp/repos/existing-repo');

		const removeButton = container.querySelector('.direct-repo-remove') as HTMLButtonElement;
		removeButton.click();
		expect(props.onRemoveDirectRepo).toHaveBeenCalledWith('/tmp/repos/existing-repo');

		const altButton = container.querySelector('.alt-chip') as HTMLButtonElement;
		altButton.click();
		expect(props.onSelectAlternative).toHaveBeenCalledWith('alt-1');

		const createButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Create',
		);
		createButton?.click();
		expect(props.onSubmit).toHaveBeenCalledTimes(1);

		unmount(component);
	});

	test('handles repos tab selection interactions', () => {
		const props = baseProps();
		props.activeTab = 'repos';
		props.selectedItems = [{ type: 'alias', name: 'repo-alias', pending: false }];

		const component = mount(WorkspaceActionCreateForm, {
			target: container,
			props,
		});

		const searchInput = container.querySelector(
			'input[placeholder="Search repos..."]',
		) as HTMLInputElement;
		searchInput.value = 'repo';
		searchInput.dispatchEvent(new Event('input', { bubbles: true }));
		expect(props.onSearchQueryInput).toHaveBeenCalledWith('repo');

		const aliasCheckbox = container.querySelector(
			'.checkbox-item input[type="checkbox"]',
		) as HTMLInputElement;
		aliasCheckbox.click();
		expect(props.onToggleAlias).toHaveBeenCalledWith('repo-alias');

		const removeButton = container.querySelector('.selected-remove') as HTMLButtonElement;
		removeButton.click();
		expect(props.onRemoveAlias).toHaveBeenCalledWith('repo-alias');

		unmount(component);
	});

	test('handles groups tab and pending selected items', () => {
		const props = baseProps();
		props.activeTab = 'groups';
		props.selectedItems = [
			{ type: 'repo', name: 'pending-repo', url: '/tmp/repos/pending-repo', pending: true },
		];

		const component = mount(WorkspaceActionCreateForm, {
			target: container,
			props,
		});

		const groupCheckbox = container.querySelector(
			'.group-card input[type="checkbox"]',
		) as HTMLInputElement;
		groupCheckbox.click();
		expect(props.onToggleGroup).toHaveBeenCalledWith('team-group');

		const expandButton = container.querySelector('.group-expand') as HTMLButtonElement;
		expandButton.click();
		expect(props.onToggleGroupExpand).toHaveBeenCalledWith('team-group');

		expect(container).toHaveTextContent('pending');
		expect(container.querySelector('.selected-remove')).toBeNull();

		unmount(component);
	});
});
