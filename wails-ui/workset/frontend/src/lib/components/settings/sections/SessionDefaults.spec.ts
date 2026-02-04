/**
 * @vitest-environment jsdom
 */
import { describe, test, expect, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import SessionDefaults from './SessionDefaults.svelte';
import type { SettingsDefaults } from '../../../types';

const buildDefaults = (): SettingsDefaults => ({
	sessionBackend: 'local',
	sessionNameFormat: '{workspace}',
	sessionTheme: 'default',
	sessionTmuxStyle: '',
	sessionTmuxLeft: '',
	sessionTmuxRight: '',
	sessionScreenHard: '',
	remote: 'origin',
	baseBranch: 'main',
	workspace: 'default',
	workspaceRoot: '/workspaces',
	repoStoreRoot: '/repos',
	agent: 'default',
	agentModel: '',
	agentLaunch: 'auto',
	terminalIdleTimeout: '0',
	terminalProtocolLog: 'off',
	terminalDebugOverlay: 'off',
});

describe('SessionDefaults', () => {
	test('renders reset layout button with tooltip', () => {
		const defaults = buildDefaults();
		const { getByText } = render(SessionDefaults, {
			props: {
				draft: defaults,
				baseline: defaults,
				onUpdate: vi.fn(),
				onRestartSessiond: vi.fn(),
				onResetTerminalLayout: vi.fn(),
			},
		});

		const button = getByText('Reset terminal layout');
		expect(button).toHaveAttribute('title', 'Resets layout and stops running terminal sessions.');
	});

	test('shows resetting state', () => {
		const defaults = buildDefaults();
		const { getByText } = render(SessionDefaults, {
			props: {
				draft: defaults,
				baseline: defaults,
				onUpdate: vi.fn(),
				onRestartSessiond: vi.fn(),
				onResetTerminalLayout: vi.fn(),
				resettingTerminalLayout: true,
			},
		});

		expect(getByText('Resetting…')).toBeInTheDocument();
	});
});
