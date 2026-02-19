import { describe, expect, it } from 'vitest';
import { formatWorkspaceActionError } from './workspaceActionErrors';

describe('formatWorkspaceActionError', () => {
	it('returns plain Error messages', () => {
		const result = formatWorkspaceActionError(new Error('simple failure'), 'fallback');
		expect(result).toBe('simple failure');
	});

	it('extracts message from JSON-wrapped Error message payloads', () => {
		const error = new Error(
			'{"message":"workset.yaml already exists at /tmp/demo/workset.yaml","kind":"RuntimeError"}',
		);
		const result = formatWorkspaceActionError(error, 'fallback');
		expect(result).toBe('workset.yaml already exists at /tmp/demo/workset.yaml');
	});

	it('extracts message from direct JSON string payloads', () => {
		const result = formatWorkspaceActionError(
			'{"message":"workspace \\"demo\\" already exists","kind":"RuntimeError"}',
			'fallback',
		);
		expect(result).toBe('workspace "demo" already exists');
	});

	it('extracts nested object message payloads', () => {
		const result = formatWorkspaceActionError(
			{
				message:
					'{"message":"workspace \\"demo\\" already exists at /tmp/demo","kind":"RuntimeError"}',
			},
			'fallback',
		);
		expect(result).toBe('workspace "demo" already exists at /tmp/demo');
	});

	it('uses fallback when no usable message exists', () => {
		expect(formatWorkspaceActionError(null, 'fallback')).toBe('fallback');
		expect(formatWorkspaceActionError({ kind: 'RuntimeError' }, 'fallback')).toBe('fallback');
		expect(formatWorkspaceActionError('   ', 'fallback')).toBe('fallback');
	});
});
