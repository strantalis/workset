import { describe, expect, test } from 'vitest';
import { decodeWailsBindingFailure } from './wailsBindingDiagnostics';

describe('decodeWailsBindingFailure', () => {
	test('extracts known Wails binding details from runtime transport payloads', () => {
		const detail = decodeWailsBindingFailure({
			url: 'http://localhost:34115/wails/runtime',
			body: JSON.stringify({
				object: 0,
				method: 0,
				args: {
					'call-id': 'abc123',
					methodID: 440589369,
					args: [{ source: 'git@github.com:acme/widgets.git' }],
				},
			}),
			status: 422,
			responseText: 'AUTH_REQUIRED: GitHub authentication required',
		});

		expect(detail).toEqual({
			status: 422,
			bindingName: 'PreviewRepoHooks',
			methodID: 440589369,
			runtimeObject: 0,
			runtimeMethod: 0,
			responseText: 'AUTH_REQUIRED: GitHub authentication required',
			argsPreview: [{ source: 'git@github.com:acme/widgets.git' }],
		});
	});

	test('ignores non-runtime failures', () => {
		const detail = decodeWailsBindingFailure({
			url: 'http://localhost:34115/api/health',
			body: JSON.stringify({ ok: false }),
			status: 500,
			responseText: 'boom',
		});

		expect(detail).toBeNull();
	});
});
