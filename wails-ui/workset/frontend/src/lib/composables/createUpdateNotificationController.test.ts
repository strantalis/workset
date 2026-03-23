import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createUpdateNotificationController } from './createUpdateNotificationController.svelte';
import type { UpdateCheckResult, UpdatePreferences, UpdateState } from '../types';

const basePreferences: UpdatePreferences = {
	channel: 'stable',
	autoCheck: true,
	dismissedVersion: '',
};

const baseUpdateState: UpdateState = {
	phase: 'idle',
	channel: 'stable',
	currentVersion: 'v1.0.0',
	latestVersion: '',
	message: '',
	error: '',
	checkedAt: '',
};

const availableUpdate: UpdateCheckResult = {
	status: 'update_available',
	channel: 'stable',
	currentVersion: 'v1.0.0',
	latestVersion: 'v1.1.0',
	message: 'Update available: v1.1.0',
	release: {
		version: 'v1.1.0',
		pubDate: '2026-03-22T00:00:00Z',
		notesUrl: 'https://github.com/anomalyco/workset/releases/tag/v1.1.0',
		minimumVersion: '',
		asset: {
			name: 'workset-v1.1.0.zip',
			url: 'https://example.com/workset.zip',
			sha256: 'abc123',
		},
		signing: {
			teamId: 'ABCDE12345',
		},
	},
};

describe('createUpdateNotificationController', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('checks immediately and every six hours when auto-check is enabled', async () => {
		const checkForUpdates = vi.fn().mockResolvedValue(availableUpdate);
		const controller = createUpdateNotificationController({
			fetchUpdatePreferences: vi.fn().mockResolvedValue(basePreferences),
			fetchUpdateState: vi.fn().mockResolvedValue(baseUpdateState),
			checkForUpdates,
			setInterval: vi.fn((handler, timeoutMs) => setInterval(handler, timeoutMs)),
			clearInterval: vi.fn((timer) => clearInterval(timer)),
		});

		await controller.init();
		expect(checkForUpdates).toHaveBeenCalledTimes(1);
		expect(controller.card?.latestVersion).toBe('v1.1.0');

		await vi.advanceTimersByTimeAsync(6 * 60 * 60 * 1000);
		expect(checkForUpdates).toHaveBeenCalledTimes(2);

		controller.destroy();
	});

	it('suppresses the card when the latest version has already been dismissed', async () => {
		const controller = createUpdateNotificationController({
			fetchUpdatePreferences: vi.fn().mockResolvedValue({
				...basePreferences,
				dismissedVersion: 'v1.1.0',
			}),
			fetchUpdateState: vi.fn().mockResolvedValue(baseUpdateState),
			checkForUpdates: vi.fn().mockResolvedValue(availableUpdate),
			setInterval: vi.fn((handler, timeoutMs) => setInterval(handler, timeoutMs)),
			clearInterval: vi.fn((timer) => clearInterval(timer)),
		});

		await controller.init();
		expect(controller.card).toBeNull();
		controller.destroy();
	});

	it('dismisses the current version and hides the card', async () => {
		const setUpdatePreferences = vi.fn().mockResolvedValue({
			...basePreferences,
			dismissedVersion: 'v1.1.0',
		});
		const controller = createUpdateNotificationController({
			fetchUpdatePreferences: vi.fn().mockResolvedValue(basePreferences),
			fetchUpdateState: vi.fn().mockResolvedValue(baseUpdateState),
			checkForUpdates: vi.fn().mockResolvedValue(availableUpdate),
			setUpdatePreferences,
			setInterval: vi.fn((handler, timeoutMs) => setInterval(handler, timeoutMs)),
			clearInterval: vi.fn((timer) => clearInterval(timer)),
		});

		await controller.init();
		expect(controller.card).not.toBeNull();

		await controller.dismiss();
		expect(setUpdatePreferences).toHaveBeenCalledWith({ dismissedVersion: 'v1.1.0' });
		expect(controller.card).toBeNull();
		controller.destroy();
	});

	it('re-checks immediately when auto-check is re-enabled', async () => {
		const checkForUpdates = vi
			.fn()
			.mockResolvedValueOnce(availableUpdate)
			.mockResolvedValueOnce({ ...availableUpdate, latestVersion: 'v1.2.0' });
		const controller = createUpdateNotificationController({
			fetchUpdatePreferences: vi.fn().mockResolvedValue({
				...basePreferences,
				autoCheck: false,
			}),
			fetchUpdateState: vi.fn().mockResolvedValue(baseUpdateState),
			checkForUpdates,
			setInterval: vi.fn((handler, timeoutMs) => setInterval(handler, timeoutMs)),
			clearInterval: vi.fn((timer) => clearInterval(timer)),
		});

		await controller.init();
		expect(checkForUpdates).not.toHaveBeenCalled();

		await controller.applyPreferences({
			...basePreferences,
			autoCheck: true,
		});
		expect(checkForUpdates).toHaveBeenCalledTimes(1);
		controller.destroy();
	});
});
