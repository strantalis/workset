import { describe, expect, it } from 'vitest';
import type { Workspace } from '../types';
import { deriveHotWorksetIds, deriveWatchedWorkspaces, rememberWorksetId } from './repoWatchScope';

const buildWorkspace = (
	id: string,
	name: string,
	worksetKey: string,
	options: Partial<Workspace> = {},
): Workspace => ({
	id,
	name,
	path: `/tmp/${id}`,
	archived: false,
	pinned: false,
	pinOrder: 0,
	expanded: false,
	lastUsed: '2026-03-22T00:00:00Z',
	worksetKey,
	worksetLabel: worksetKey,
	repos: [],
	...options,
});

describe('repoWatchScope', () => {
	it('keeps the active workset hot and includes all of its threads', () => {
		const workspaces = [
			buildWorkspace('thread-a', 'Alpha 1', 'alpha'),
			buildWorkspace('thread-b', 'Alpha 2', 'alpha'),
			buildWorkspace('thread-c', 'Beta 1', 'beta'),
		];

		expect(
			deriveWatchedWorkspaces({
				workspaces,
				activeWorkspaceId: 'thread-b',
			}).map((workspace) => workspace.id),
		).toEqual(['thread-a', 'thread-b']);
	});

	it('preserves a small warm cache of recently visited worksets', () => {
		const workspaces = [
			buildWorkspace('thread-a', 'Alpha 1', 'alpha'),
			buildWorkspace('thread-b', 'Beta 1', 'beta'),
			buildWorkspace('thread-c', 'Gamma 1', 'gamma'),
		];

		const warm = rememberWorksetId(rememberWorksetId([], 'beta'), 'gamma');
		expect([
			...deriveHotWorksetIds({ workspaces, activeWorkspaceId: 'thread-a', warmWorksetIds: warm }),
		]).toEqual(['alpha', 'gamma', 'beta']);
	});

	it('drops archived and placeholder threads from the watch scope', () => {
		const workspaces = [
			buildWorkspace('thread-a', 'Alpha 1', 'alpha'),
			buildWorkspace('thread-b', 'Alpha 2', 'alpha', { archived: true }),
			buildWorkspace('thread-c', 'Alpha Placeholder', 'alpha', { placeholder: true }),
			buildWorkspace('thread-d', 'Beta 1', 'beta'),
		];

		expect(
			deriveWatchedWorkspaces({
				workspaces,
				activeWorkspaceId: 'thread-a',
				warmWorksetIds: ['beta'],
			}).map((workspace) => workspace.id),
		).toEqual(['thread-a', 'thread-d']);
	});

	it('caps the warm cache and de-duplicates repeated workset visits', () => {
		let warm: string[] = [];
		warm = rememberWorksetId(warm, 'alpha');
		warm = rememberWorksetId(warm, 'beta');
		warm = rememberWorksetId(warm, 'alpha');
		warm = rememberWorksetId(warm, 'gamma');
		warm = rememberWorksetId(warm, 'delta');

		expect(warm).toEqual(['delta', 'gamma', 'alpha']);
	});
});
