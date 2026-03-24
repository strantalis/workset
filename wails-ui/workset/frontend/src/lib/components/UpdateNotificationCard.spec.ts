import { render } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import UpdateNotificationCard from './UpdateNotificationCard.svelte';

describe('UpdateNotificationCard', () => {
	it('renders version, release notes, and actions for an available update', () => {
		const { getByText, getByRole } = render(UpdateNotificationCard, {
			props: {
				notification: {
					mode: 'available',
					latestVersion: 'v1.1.0',
					message: 'Update available: v1.1.0',
					notesUrl: 'https://github.com/anomalyco/workset/releases/tag/v1.1.0',
					error: null,
				},
				onDismiss: vi.fn(),
				onUpdate: vi.fn(),
			},
		});

		expect(getByText('v1.1.0 available')).toBeInTheDocument();
		expect(getByText('Update available: v1.1.0')).toBeInTheDocument();
		expect(getByRole('link', { name: /Release Notes/i })).toHaveAttribute(
			'href',
			'https://github.com/anomalyco/workset/releases/tag/v1.1.0',
		);
		expect(getByRole('button', { name: 'Dismiss' })).toBeInTheDocument();
		expect(getByRole('button', { name: 'Update & Restart' })).toBeInTheDocument();
	});

	it('hides action buttons while an update is applying', () => {
		const { queryByRole, getByText } = render(UpdateNotificationCard, {
			props: {
				notification: {
					mode: 'applying',
					latestVersion: 'v1.1.0',
					message: 'Applying update. The app will restart shortly.',
					notesUrl: 'https://github.com/anomalyco/workset/releases/tag/v1.1.0',
					error: null,
				},
				onDismiss: vi.fn(),
				onUpdate: vi.fn(),
			},
		});

		expect(getByText('Installing update v1.1.0…')).toBeInTheDocument();
		expect(getByText('Applying update. The app will restart shortly.')).toBeInTheDocument();
		expect(queryByRole('button', { name: 'Dismiss' })).toBeNull();
		expect(queryByRole('button', { name: 'Update & Restart' })).toBeNull();
	});
});
