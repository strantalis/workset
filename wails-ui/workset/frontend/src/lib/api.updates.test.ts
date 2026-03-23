import { beforeEach, describe, expect, test, vi } from 'vitest';
import { setUpdatePreferences } from './api/updates';
import { SetUpdatePreferences } from '../../bindings/workset/app';

vi.mock('../../bindings/workset/app', () => ({
	SetUpdatePreferences: vi.fn(),
}));

describe('setUpdatePreferences', () => {
	beforeEach(() => {
		vi.mocked(SetUpdatePreferences).mockResolvedValue({
			channel: 'stable',
			autoCheck: true,
			dismissedVersion: '',
		});
	});

	test('does not force channel when only autoCheck is provided', async () => {
		await setUpdatePreferences({ autoCheck: false });
		expect(SetUpdatePreferences).toHaveBeenCalledWith({
			channel: '',
			autoCheck: false,
			dismissedVersion: null,
		});
	});

	test('passes channel when provided', async () => {
		await setUpdatePreferences({ channel: 'alpha' });
		expect(SetUpdatePreferences).toHaveBeenCalledWith({
			channel: 'alpha',
			autoCheck: null,
			dismissedVersion: null,
		});
	});

	test('passes both fields when both are provided', async () => {
		await setUpdatePreferences({ channel: 'alpha', autoCheck: false });
		expect(SetUpdatePreferences).toHaveBeenCalledWith({
			channel: 'alpha',
			autoCheck: false,
			dismissedVersion: null,
		});
	});

	test('passes dismissedVersion when provided', async () => {
		await setUpdatePreferences({ dismissedVersion: 'v1.2.3' });
		expect(SetUpdatePreferences).toHaveBeenCalledWith({
			channel: '',
			autoCheck: null,
			dismissedVersion: 'v1.2.3',
		});
	});
});
