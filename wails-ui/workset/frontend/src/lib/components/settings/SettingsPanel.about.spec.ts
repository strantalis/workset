import { describe, test, expect, vi, afterEach, beforeEach } from 'vitest';
import { render, fireEvent, cleanup, waitFor } from '@testing-library/svelte';
import SettingsPanel from '../SettingsPanel.svelte';
import * as settingsApi from '../../api/settings';
import * as updatesApi from '../../api/updates';
import * as terminalApi from '../../api/terminal-layout';
import type { SettingsDefaults } from '../../types';

const api = {
	...settingsApi,
	...updatesApi,
	...terminalApi,
};

vi.mock('../../api/settings', () => ({
	fetchSettings: vi.fn(),
	setDefaultSetting: vi.fn(),
	restartSessiond: vi.fn(),
}));

vi.mock('../../api/updates', () => ({
	fetchAppVersion: vi.fn(),
	fetchUpdatePreferences: vi.fn(),
	fetchUpdateState: vi.fn(),
	checkForUpdates: vi.fn(),
	startAppUpdate: vi.fn(),
	setUpdatePreferences: vi.fn(),
}));

vi.mock('../../api/terminal-layout', () => ({
	fetchWorkspaceTerminalLayout: vi.fn(),
	createWorkspaceTerminal: vi.fn(),
	persistWorkspaceTerminalLayout: vi.fn(),
	stopWorkspaceTerminal: vi.fn(),
}));

describe('SettingsPanel About Section', () => {
	const mockOnClose = vi.fn();
	const baseDefaults: SettingsDefaults = {
		remote: 'origin',
		baseBranch: 'main',
		workspace: 'test',
		workspaceRoot: '/workspaces',
		repoStoreRoot: '/repos',
		sessionBackend: 'local',
		sessionNameFormat: '{workspace}',
		sessionTheme: 'default',
		sessionTmuxStyle: '',
		sessionTmuxLeft: '',
		sessionTmuxRight: '',
		sessionScreenHard: '',
		agent: 'default',
		agentModel: '',
		terminalIdleTimeout: '0',
		terminalProtocolLog: 'off',
		terminalDebugOverlay: 'off',
	};
	const buildDefaults = (overrides: Partial<SettingsDefaults> = {}): SettingsDefaults => ({
		...baseDefaults,
		...overrides,
	});

	beforeEach(() => {
		vi.mocked(api.fetchUpdatePreferences).mockResolvedValue({
			channel: 'stable',
			autoCheck: true,
		});
		vi.mocked(api.fetchUpdateState).mockResolvedValue({
			phase: 'idle',
			channel: 'stable',
			currentVersion: '',
			latestVersion: '',
			message: '',
			error: '',
			checkedAt: '',
		});
	});

	afterEach(() => {
		cleanup();
		vi.clearAllMocks();
	});

	const waitForLoadingAndClickAbout = async (
		getByText: (text: string) => HTMLElement,
		queryByText: (text: string) => HTMLElement | null,
	) => {
		// Wait for loading to finish
		await waitFor(() => {
			expect(queryByText('Loading settings...')).not.toBeInTheDocument();
		});

		// Wait for About button to appear in sidebar
		await waitFor(() => {
			expect(getByText('About')).toBeInTheDocument();
		});

		// Click on About
		const aboutButton = getByText('About');
		await fireEvent.click(aboutButton);

		// Wait for About content to render
		await waitFor(() => {
			expect(getByText('Workset')).toBeInTheDocument();
		});
	};

	test('renders About section when about is selected', async () => {
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});

		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.0.0',
			commit: 'abc123',
			dirty: false,
		});

		const { getByText, queryByText } = render(SettingsPanel, {
			props: {
				onClose: mockOnClose,
			},
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		// Check About section content
		expect(getByText('Workspace management for multi-repo development')).toBeInTheDocument();
		expect(getByText('Version')).toBeInTheDocument();
		expect(getByText('GitHub Repository')).toBeInTheDocument();
		expect(getByText('Report an Issue')).toBeInTheDocument();
	});

	test('displays version information correctly', async () => {
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});

		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.2.3',
			commit: 'def456',
			dirty: true,
		});

		const { getByText, queryByText } = render(SettingsPanel, {
			props: {
				onClose: mockOnClose,
			},
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		// Check version info
		expect(getByText('1.2.3+dirty')).toBeInTheDocument();
		expect(getByText('def456')).toBeInTheDocument();
	});

	test('truncates long commit hash for display while keeping full value in title', async () => {
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});

		const longCommit = '5e8f013aa0e9305c47cdacfb6b77bc6784c128f6';
		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.2.3',
			commit: longCommit,
			dirty: false,
		});

		const { getByText, queryByText } = render(SettingsPanel, {
			props: {
				onClose: mockOnClose,
			},
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		const commitText = getByText('5e8f013aa0e9');
		expect(commitText).toBeInTheDocument();
		expect(commitText).toHaveAttribute('title', longCommit);
	});

	test('displays version as dev when version is dev', async () => {
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});

		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: 'dev',
			commit: '',
			dirty: false,
		});

		const { getByText, queryByText } = render(SettingsPanel, {
			props: {
				onClose: mockOnClose,
			},
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		// Check dev version is displayed
		expect(getByText('dev')).toBeInTheDocument();
	});

	test('renders About section even when version fetch fails', async () => {
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});

		vi.mocked(api.fetchAppVersion).mockRejectedValue(new Error('Failed to fetch'));

		const { getByText, queryByText } = render(SettingsPanel, {
			props: {
				onClose: mockOnClose,
			},
		});

		// Wait for loading to finish
		await waitFor(() => {
			expect(queryByText('Loading settings...')).not.toBeInTheDocument();
		});

		// Wait for About button to appear in sidebar
		await waitFor(() => {
			expect(getByText('About')).toBeInTheDocument();
		});

		// Click on About
		const aboutButton = getByText('About');
		await fireEvent.click(aboutButton);

		// About section should still render even without version info
		// Look for elements that are always present.
		await waitFor(() => {
			expect(getByText('GitHub Repository')).toBeInTheDocument();
		});

		// Version section should NOT be present when appVersion is null
		expect(queryByText('Version')).not.toBeInTheDocument();
	});

	test('displays about action links', async () => {
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});

		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.0.0',
			commit: 'abc123',
			dirty: false,
		});

		const { getByText, queryByText } = render(SettingsPanel, {
			props: {
				onClose: mockOnClose,
			},
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		expect(getByText('GitHub Repository')).toBeInTheDocument();
		expect(getByText('Report an Issue')).toBeInTheDocument();
	});

	test('displays GitHub links', async () => {
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});

		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.0.0',
			commit: 'abc123',
			dirty: false,
		});

		const { getByText, queryByText } = render(SettingsPanel, {
			props: {
				onClose: mockOnClose,
			},
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		// Check links
		expect(getByText('GitHub Repository')).toBeInTheDocument();
		expect(getByText('Report an Issue')).toBeInTheDocument();
	});

	test('shows backend string error when update check fails', async () => {
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});
		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.0.0',
			commit: 'abc123',
			dirty: false,
		});
		vi.mocked(api.checkForUpdates).mockRejectedValue('network timeout while fetching manifest');

		const { getByText, queryByText } = render(SettingsPanel, {
			props: {
				onClose: mockOnClose,
			},
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		await fireEvent.click(getByText('Check for Updates'));

		await waitFor(() => {
			expect(getByText('network timeout while fetching manifest')).toBeInTheDocument();
		});
		expect(queryByText('Failed to update settings.')).not.toBeInTheDocument();
	});

	test('shows backend object message when update check fails', async () => {
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});
		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.0.0',
			commit: 'abc123',
			dirty: false,
		});
		vi.mocked(api.checkForUpdates).mockRejectedValue({
			message: 'updates endpoint returned 503',
		});

		const { getByText, queryByText } = render(SettingsPanel, {
			props: {
				onClose: mockOnClose,
			},
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		await fireEvent.click(getByText('Check for Updates'));

		await waitFor(() => {
			expect(getByText('updates endpoint returned 503')).toBeInTheDocument();
		});
	});

	test('copy button copies version info to clipboard', async () => {
		const clipboardWriteText = vi.fn();
		Object.assign(navigator, {
			clipboard: {
				writeText: clipboardWriteText,
			},
		});

		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});

		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.0.0',
			commit: 'abc123',
			dirty: false,
		});

		const { getByText, getByTitle, queryByText } = render(SettingsPanel, {
			props: {
				onClose: mockOnClose,
			},
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		// Click copy button
		const copyButton = getByTitle('Copy version info');
		await fireEvent.click(copyButton);

		expect(clipboardWriteText).toHaveBeenCalledWith('Workset 1.0.0 (abc123)');
	});

	test('commit copy button copies full SHA to clipboard', async () => {
		const clipboardWriteText = vi.fn().mockResolvedValue(undefined);
		Object.assign(navigator, {
			clipboard: { writeText: clipboardWriteText },
		});

		const fullCommit = 'cdadc66bb8c1234567890abcdef';
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});
		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.0.0',
			commit: fullCommit,
			dirty: false,
		});

		const { getByText, getByTitle, queryByText } = render(SettingsPanel, {
			props: { onClose: mockOnClose },
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		const copyCommitButton = getByTitle('Copy commit SHA');
		await fireEvent.click(copyCommitButton);

		expect(clipboardWriteText).toHaveBeenCalledWith(fullCommit);
	});

	test('commit copy button shows Copied! tooltip after click', async () => {
		vi.useFakeTimers();
		const clipboardWriteText = vi.fn().mockResolvedValue(undefined);
		Object.assign(navigator, {
			clipboard: { writeText: clipboardWriteText },
		});

		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});
		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.0.0',
			commit: 'abc123def456',
			dirty: false,
		});

		const { getByText, getByTitle, queryByText, queryByTitle } = render(SettingsPanel, {
			props: { onClose: mockOnClose },
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		await fireEvent.click(getByTitle('Copy commit SHA'));

		// Tooltip should now say "Copied!"
		await waitFor(() => {
			expect(queryByTitle('Copied!')).toBeInTheDocument();
		});

		// After 1500ms the tooltip should revert
		vi.advanceTimersByTime(1500);
		await waitFor(() => {
			expect(queryByTitle('Copy commit SHA')).toBeInTheDocument();
		});

		vi.useRealTimers();
	});

	test('rapid commit copy clicks do not cause premature reset of copied state', async () => {
		vi.useFakeTimers();
		const clipboardWriteText = vi.fn().mockResolvedValue(undefined);
		Object.assign(navigator, {
			clipboard: { writeText: clipboardWriteText },
		});

		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});
		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.0.0',
			commit: 'abc123def456',
			dirty: false,
		});

		const { getByText, getByTitle, queryByText, queryByTitle } = render(SettingsPanel, {
			props: { onClose: mockOnClose },
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		// Click once, advance partway, click again
		await fireEvent.click(getByTitle('Copy commit SHA'));
		await waitFor(() => expect(queryByTitle('Copied!')).toBeInTheDocument());

		vi.advanceTimersByTime(800); // before first timer fires
		await fireEvent.click(getByTitle('Copied!')); // second click resets timer

		// Should still show "Copied!" — first timer was cleared
		vi.advanceTimersByTime(800); // 800ms into second timer, still within window
		await waitFor(() => expect(queryByTitle('Copied!')).toBeInTheDocument());

		// After full 1500ms from second click it should revert
		vi.advanceTimersByTime(700);
		await waitFor(() => expect(queryByTitle('Copy commit SHA')).toBeInTheDocument());

		vi.useRealTimers();
	});

	test('commit copy button is not rendered when commit is absent', async () => {
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});
		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: 'dev',
			commit: '',
			dirty: false,
		});

		const { getByText, queryByText, queryByTitle } = render(SettingsPanel, {
			props: { onClose: mockOnClose },
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		expect(queryByTitle('Copy commit SHA')).not.toBeInTheDocument();
	});

	test('displays copyright with current year', async () => {
		vi.mocked(api.fetchSettings).mockResolvedValue({
			configPath: '/test/config.yaml',
			defaults: buildDefaults(),
		});

		vi.mocked(api.fetchAppVersion).mockResolvedValue({
			version: '1.0.0',
			commit: 'abc123',
			dirty: false,
		});

		const { getByText, queryByText } = render(SettingsPanel, {
			props: {
				onClose: mockOnClose,
			},
		});

		await waitForLoadingAndClickAbout(getByText, queryByText);

		// Check copyright with current year
		const currentYear = new Date().getFullYear();
		expect(
			getByText(`© ${currentYear} Sean Trantalis. Open source under MIT License.`),
		).toBeInTheDocument();
	});
});
