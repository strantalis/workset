/**
 * @vitest-environment jsdom
 */
import { describe, test, expect, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import SessionDefaults from './SessionDefaults.svelte';
import type { SettingsDefaults } from '../../../types';

const buildDefaults = (): SettingsDefaults => ({
	remote: 'origin',
	baseBranch: 'main',
	thread: 'default',
	worksetRoot: '/workset',
	repoStoreRoot: '/repos',
	agent: 'default',
	agentModel: '',
	terminalIdleTimeout: '0',
	terminalDebugLog: 'off',
	terminalProtocolLog: 'off',
	terminalDebugOverlay: 'off',
	terminalFontSize: '13',
	terminalCursorBlink: 'on',
});

describe('SessionDefaults', () => {
	test('renders terminal preference fields', () => {
		const defaults = buildDefaults();
		const { getByText } = render(SessionDefaults, {
			props: {
				draft: defaults,
				baseline: defaults,
				onUpdate: vi.fn(),
			},
		});

		expect(getByText('Lifecycle debug log')).toBeInTheDocument();
		expect(getByText('Protocol logging')).toBeInTheDocument();
		expect(getByText('Debug overlay')).toBeInTheDocument();
		expect(getByText('Idle timeout')).toBeInTheDocument();
		expect(getByText('Text size')).toBeInTheDocument();
		expect(getByText('Cursor blink')).toBeInTheDocument();
	});

	test('renders section title', () => {
		const defaults = buildDefaults();
		const { getByText } = render(SessionDefaults, {
			props: {
				draft: defaults,
				baseline: defaults,
				onUpdate: vi.fn(),
			},
		});

		expect(getByText('Terminal')).toBeInTheDocument();
	});

	test('renders text size as a number input with spinner-friendly bounds', () => {
		const defaults = buildDefaults();
		const { getByLabelText } = render(SessionDefaults, {
			props: {
				draft: defaults,
				baseline: defaults,
				onUpdate: vi.fn(),
			},
		});

		const textSizeInput = getByLabelText('Text size') as HTMLInputElement;

		expect(textSizeInput.type).toBe('number');
		expect(textSizeInput.min).toBe('8');
		expect(textSizeInput.max).toBe('28');
		expect(textSizeInput.step).toBe('1');
	});
});
