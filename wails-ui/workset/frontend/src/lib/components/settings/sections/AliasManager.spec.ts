import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import AliasManager from './AliasManager.svelte';
import * as settingsService from '../../../api/settings';

vi.mock('../../../api/settings', () => ({
	listAliases: vi.fn(),
	createAlias: vi.fn(),
	updateAlias: vi.fn(),
	deleteAlias: vi.fn(),
	openDirectoryDialog: vi.fn(),
}));

describe('AliasManager', () => {
	beforeEach(() => {
		vi.mocked(settingsService.listAliases).mockResolvedValue([]);
	});

	afterEach(() => {
		cleanup();
		vi.clearAllMocks();
	});

	test('auto-fills repo name from URL when name field is empty', async () => {
		const { getByLabelText, getByText } = render(AliasManager, {
			props: {
				onAliasCountChange: vi.fn(),
			},
		});

		await fireEvent.click(getByText('Register Repo'));

		const nameInput = getByLabelText('Name') as HTMLInputElement;
		const sourceInput = getByLabelText('Source (URL or path)') as HTMLInputElement;

		expect(nameInput).toHaveValue('');
		await fireEvent.input(sourceInput, {
			target: { value: 'https://github.com/acme/widget-service.git' },
		});

		expect(nameInput).toHaveValue('widget-service');
	});

	test('does not overwrite a user-provided name when source URL is entered', async () => {
		const { getByLabelText, getByText } = render(AliasManager, {
			props: {
				onAliasCountChange: vi.fn(),
			},
		});

		await fireEvent.click(getByText('Register Repo'));

		const nameInput = getByLabelText('Name') as HTMLInputElement;
		const sourceInput = getByLabelText('Source (URL or path)') as HTMLInputElement;

		await fireEvent.input(nameInput, {
			target: { value: 'custom-name' },
		});
		await fireEvent.input(sourceInput, {
			target: { value: 'https://github.com/acme/widget-service.git' },
		});

		expect(nameInput).toHaveValue('custom-name');
	});
});
