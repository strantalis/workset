import { afterEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import ContextBar from './ContextBar.svelte';
import type { WorksetSummary } from '../../view-models/worksetViewModel';

const buildWorkset = (overrides: Partial<WorksetSummary> = {}): WorksetSummary => ({
	id: 'ws-1',
	label: 'workspace-one',
	description: 'workspace',
	template: 'Library',
	repos: ['repo-one'],
	branch: 'main',
	repoCount: 1,
	dirtyCount: 0,
	openPrs: 0,
	mergedPrs: 0,
	linesAdded: 0,
	linesRemoved: 0,
	lastActive: 'just now',
	lastActiveTs: Date.now(),
	health: ['clean'],
	pinned: false,
	archived: false,
	...overrides,
});

describe('ContextBar', () => {
	afterEach(() => {
		cleanup();
	});

	test('shows popout action and calls toggle handler when enabled', async () => {
		const onTogglePopout = vi.fn();
		const { getByRole } = render(ContextBar, {
			props: {
				workset: buildWorkset(),
				onOpenHub: vi.fn(),
				showPaletteHint: false,
				showPopoutToggle: true,
				workspacePoppedOut: false,
				onTogglePopout,
			},
		});

		const popoutButton = getByRole('button', { name: 'Open workspace popout' });
		expect(popoutButton).toBeInTheDocument();
		expect(popoutButton.textContent).toContain('Popout');

		await fireEvent.click(popoutButton);
		expect(onTogglePopout).toHaveBeenCalledTimes(1);
	});

	test('shows return action when workspace is already popped out', () => {
		const { getByRole } = render(ContextBar, {
			props: {
				workset: buildWorkset(),
				onOpenHub: vi.fn(),
				showPaletteHint: false,
				showPopoutToggle: true,
				workspacePoppedOut: true,
				onTogglePopout: vi.fn(),
			},
		});

		const returnButton = getByRole('button', { name: 'Return workspace to main window' });
		expect(returnButton).toBeInTheDocument();
		expect(returnButton.textContent).toContain('Return');
	});

	test('does not render popout action when disabled', () => {
		const { queryByRole } = render(ContextBar, {
			props: {
				workset: buildWorkset(),
				onOpenHub: vi.fn(),
				showPaletteHint: false,
				showPopoutToggle: false,
			},
		});

		expect(queryByRole('button', { name: 'Open workspace popout' })).toBeNull();
		expect(queryByRole('button', { name: 'Return workspace to main window' })).toBeNull();
	});
});
