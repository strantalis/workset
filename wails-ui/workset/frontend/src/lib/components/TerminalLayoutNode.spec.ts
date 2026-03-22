import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('./TerminalPane.svelte', async () => {
	const module = await import('./test-utils/MockTerminalPane.svelte');
	return { default: module.default };
});

import TerminalLayoutNode from './TerminalLayoutNode.svelte';

describe('TerminalLayoutNode', () => {
	afterEach(() => {
		cleanup();
	});

	it('opens a pane-chrome context menu and splits from it', async () => {
		const onFocusPane = vi.fn();
		const onClosePane = vi.fn();
		const onSplitPane = vi.fn();

		const { container } = render(TerminalLayoutNode, {
			props: {
				node: {
					id: 'pane-1',
					kind: 'pane',
					terminalId: 'term-1',
				},
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				focusedPaneId: 'pane-1',
				onFocusPane,
				onClosePane,
				onSplitPane,
			},
		});

		const pane = container.querySelector('.pane');
		expect(pane).toBeTruthy();
		await fireEvent.contextMenu(pane!);

		expect(onFocusPane).toHaveBeenCalledWith('pane-1');
		const splitVertical = await screen.findByRole('menuitem', { name: 'Split vertical' });
		await fireEvent.click(splitVertical);

		expect(onSplitPane).toHaveBeenCalledWith('pane-1', 'row');
	});

	it('closes a pane from the context menu and when the terminal exits', async () => {
		const onFocusPane = vi.fn();
		const onClosePane = vi.fn();
		const onSplitPane = vi.fn();

		const { container } = render(TerminalLayoutNode, {
			props: {
				node: {
					id: 'pane-1',
					kind: 'pane',
					terminalId: 'term-1',
				},
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				focusedPaneId: 'pane-1',
				onFocusPane,
				onClosePane,
				onSplitPane,
			},
		});

		const pane = container.querySelector('.pane');
		expect(pane).toBeTruthy();
		await fireEvent.contextMenu(pane!);

		const closeSplit = await screen.findByRole('menuitem', { name: 'Close split' });
		await fireEvent.click(closeSplit);

		expect(onClosePane).toHaveBeenCalledWith('pane-1');

		const closeFromTerminal = await screen.findByTestId('mock-terminal-close');
		await fireEvent.click(closeFromTerminal);

		expect(onClosePane).toHaveBeenCalledTimes(2);
	});
});
