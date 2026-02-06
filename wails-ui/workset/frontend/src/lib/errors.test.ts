import { describe, expect, test } from 'vitest';
import { toErrorMessage } from './errors';

describe('toErrorMessage', () => {
	test('returns Error message', () => {
		expect(toErrorMessage(new Error('boom'), 'fallback')).toBe('boom');
	});

	test('returns string rejection reason', () => {
		expect(toErrorMessage('request timed out', 'fallback')).toBe('request timed out');
	});

	test('returns object message field', () => {
		expect(toErrorMessage({ message: 'service unavailable' }, 'fallback')).toBe(
			'service unavailable',
		);
	});

	test('returns fallback for unknown shapes', () => {
		expect(toErrorMessage({ code: 503 }, 'fallback')).toBe('fallback');
		expect(toErrorMessage('', 'fallback')).toBe('fallback');
	});
});
