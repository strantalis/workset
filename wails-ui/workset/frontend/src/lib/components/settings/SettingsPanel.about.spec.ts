import { describe, test, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup, waitFor } from '@testing-library/svelte';
import SettingsPanel from '../SettingsPanel.svelte';
import * as api from '../../api';
import type { SettingsDefaults } from '../../types';

// Mock the API module
vi.mock('../../api', () => ({
	fetchSettings: vi.fn(),
	fetchAppVersion: vi.fn(),
	fetchWorkspaceTerminalLayout: vi.fn(),
	setDefaultSetting: vi.fn(),
	restartSessiond: vi.fn(),
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
		agentLaunch: 'auto',
		terminalIdleTimeout: '0',
		terminalProtocolLog: 'off',
		terminalDebugOverlay: 'off',
	};
	const buildDefaults = (overrides: Partial<SettingsDefaults> = {}): SettingsDefaults => ({
		...baseDefaults,
		...overrides,
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
		expect(getByText('Built With')).toBeInTheDocument();
		expect(getByText('Links')).toBeInTheDocument();
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
		// Look for elements that are always present (Built With section)
		await waitFor(() => {
			expect(getByText('Built With')).toBeInTheDocument();
		});

		// Version section should NOT be present when appVersion is null
		expect(queryByText('Version')).not.toBeInTheDocument();
	});

	test('displays tech stack badges', async () => {
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

		// Check tech badges
		expect(getByText('Wails')).toBeInTheDocument();
		expect(getByText('Svelte')).toBeInTheDocument();
		expect(getByText('Go')).toBeInTheDocument();
		expect(getByText('TypeScript')).toBeInTheDocument();
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
