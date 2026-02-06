import { beforeEach, describe, expect, test, vi } from 'vitest';
import { setUpdatePreferences } from './api';
import { SetUpdatePreferences } from '../../wailsjs/go/main/App';

vi.mock('../../wailsjs/go/main/App', () => ({
	SetUpdatePreferences: vi.fn(),
}));

describe('setUpdatePreferences', () => {
	beforeEach(() => {
		vi.mocked(SetUpdatePreferences).mockResolvedValue({
			channel: 'stable',
			autoCheck: true,
		});
	});

	test('does not force channel when only autoCheck is provided', async () => {
		await setUpdatePreferences({ autoCheck: false });
		expect(SetUpdatePreferences).toHaveBeenCalledWith({ channel: '', autoCheck: false });
	});

	test('passes channel when provided', async () => {
		await setUpdatePreferences({ channel: 'alpha' });
		expect(SetUpdatePreferences).toHaveBeenCalledWith({ channel: 'alpha' });
	});

	test('passes both fields when both are provided', async () => {
		await setUpdatePreferences({ channel: 'alpha', autoCheck: false });
		expect(SetUpdatePreferences).toHaveBeenCalledWith({ channel: 'alpha', autoCheck: false });
	});
});
