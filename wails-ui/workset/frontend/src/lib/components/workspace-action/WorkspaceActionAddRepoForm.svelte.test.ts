/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { mount, unmount } from 'svelte';
import type { ComponentProps } from 'svelte';
import type { Alias, GroupSummary } from '../../types';
import WorkspaceActionAddRepoForm from './WorkspaceActionAddRepoForm.svelte';

const aliases: Alias[] = [{ name: 'repo-alias', path: '/tmp/repos/repo-alias' }];
const groups: GroupSummary[] = [{ name: 'team-group', description: 'Shared repos', repo_count: 2 }];
const groupDetails = new Map<string, string[]>([['team-group', ['repo-a', 'repo-b']]]);
type AddRepoFormProps = ComponentProps<typeof WorkspaceActionAddRepoForm>;

const baseProps = (): AddRepoFormProps => ({
	loading: false,
	activeTab: 'repos',
	aliasItems: aliases,
	groupItems: groups,
	searchQuery: '',
	addSource: '',
	filteredAliases: aliases,
	filteredGroups: groups,
	selectedAliases: new Set<string>(),
	selectedGroups: new Set<string>(),
	expandedGroups: new Set<string>(),
	groupDetails,
	existingRepos: [{ name: 'repo-existing' }],
	addRepoSelectedItems: [{ type: 'alias', name: 'repo-alias' }],
	addRepoTotalItems: 1,
	getAliasSource: (alias: Alias) => alias.path || alias.url || '',
	onTabChange: vi.fn(),
	onSearchQueryInput: vi.fn(),
	onAddSourceInput: vi.fn(),
	onBrowse: vi.fn(),
	onToggleAlias: vi.fn(),
	onToggleGroup: vi.fn(),
	onToggleGroupExpand: vi.fn(),
	onRemoveAlias: vi.fn(),
	onRemoveGroup: vi.fn(),
	onSubmit: vi.fn(),
});

describe('WorkspaceActionAddRepoForm', () => {
	let container: HTMLDivElement;

	beforeEach(() => {
		container = document.createElement('div');
		document.body.appendChild(container);
	});

	afterEach(() => {
		container.remove();
	});

	test('renders existing + selected items and triggers repos-tab callbacks', () => {
		const props = baseProps();
		const component = mount(WorkspaceActionAddRepoForm, {
			target: container,
			props,
		});

		expect(container).toHaveTextContent('Already in workspace (1 repos)');
		expect(container).toHaveTextContent('Selected (1 items)');
		expect(container).toHaveTextContent('repo-alias');

		const directTab = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Direct',
		);
		directTab?.click();
		expect(props.onTabChange).toHaveBeenCalledWith('direct');

		const aliasCheckbox = container.querySelector(
			'.checkbox-item input[type="checkbox"]',
		) as HTMLInputElement;
		aliasCheckbox.click();
		expect(props.onToggleAlias).toHaveBeenCalledWith('repo-alias');

		const removeButton = container.querySelector('.selected-remove') as HTMLButtonElement;
		removeButton.click();
		expect(props.onRemoveAlias).toHaveBeenCalledWith('repo-alias');

		const submitButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Add',
		);
		submitButton?.click();
		expect(props.onSubmit).toHaveBeenCalledTimes(1);

		unmount(component);
	});

	test('triggers direct-tab source and browse callbacks', () => {
		const props = baseProps();
		props.activeTab = 'direct';
		props.addSource = '/tmp/repos/new-repo';
		props.addRepoSelectedItems = [{ type: 'repo', name: '/tmp/repos/new-repo' }];

		const component = mount(WorkspaceActionAddRepoForm, {
			target: container,
			props,
		});

		const sourceInput = container.querySelector(
			'input[placeholder="git@github.com:org/repo.git"]',
		) as HTMLInputElement;
		sourceInput.value = '/tmp/repos/updated-repo';
		sourceInput.dispatchEvent(new Event('input', { bubbles: true }));
		expect(props.onAddSourceInput).toHaveBeenCalledWith('/tmp/repos/updated-repo');

		const browseButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Browse',
		);
		browseButton?.click();
		expect(props.onBrowse).toHaveBeenCalledTimes(1);

		const removeButton = container.querySelector('.selected-remove') as HTMLButtonElement;
		removeButton.click();
		expect(props.onAddSourceInput).toHaveBeenCalledWith('');

		unmount(component);
	});

	test('disables submit when no items are selected', () => {
		const props = baseProps();
		props.addRepoSelectedItems = [];
		props.addRepoTotalItems = 0;

		const component = mount(WorkspaceActionAddRepoForm, {
			target: container,
			props,
		});

		const submitButton = Array.from(container.querySelectorAll('button')).find(
			(button) => button.textContent?.trim() === 'Add',
		) as HTMLButtonElement;
		expect(submitButton).toBeDisabled();

		unmount(component);
	});
});
