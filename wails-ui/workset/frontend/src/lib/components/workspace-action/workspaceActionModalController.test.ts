import { describe, expect, it } from 'vitest';
import {
	deriveWorkspaceActionModalSize,
	deriveWorkspaceActionModalSubtitle,
	deriveWorkspaceActionModalTitle,
	resetWorkspaceActionFlow,
	resolveMutationHookTransition,
	resolveRemovalState,
	shouldRefreshRemoveRepoStatus,
} from '../../services/workspaceActionModalController';

describe('workspaceActionModalController', () => {
	it('derives modal title, subtitle and size from mode and phase', () => {
		expect(deriveWorkspaceActionModalTitle('create', 'form')).toBe('Create workset');
		expect(deriveWorkspaceActionModalTitle('remove-repo', 'form')).toBe('Remove repo');
		expect(deriveWorkspaceActionModalTitle('rename', 'hook-results')).toBe('Hook results');

		expect(
			deriveWorkspaceActionModalSubtitle({
				phase: 'hook-results',
				mode: 'add-repo',
				workspaceName: 'alpha',
				hookResultContext: { action: 'added', name: 'alpha', itemCount: 2 },
			}),
		).toBe('alpha');
		expect(
			deriveWorkspaceActionModalSubtitle({
				phase: 'form',
				mode: 'create',
				workspaceName: 'alpha',
				hookResultContext: null,
			}),
		).toBe('');
		expect(
			deriveWorkspaceActionModalSubtitle({
				phase: 'form',
				mode: 'rename',
				workspaceName: 'alpha',
				hookResultContext: null,
			}),
		).toBe('alpha');

		expect(deriveWorkspaceActionModalSize('create', 'form')).toBe('wide');
		expect(deriveWorkspaceActionModalSize('rename', 'form')).toBe('md');
		expect(deriveWorkspaceActionModalSize('rename', 'hook-results')).toBe('md');
	});

	it('resets flow state to the form phase', () => {
		expect(resetWorkspaceActionFlow()).toEqual({
			phase: 'form',
			hookResultContext: null,
		});
	});

	it('resolves create transitions with close when no hook activity exists', () => {
		expect(
			resolveMutationHookTransition({
				action: 'created',
				workspaceName: 'alpha',
				warnings: [],
				pendingHooks: [],
				hookRuns: [],
			}),
		).toEqual({
			phase: 'form',
			hookResultContext: null,
			success: null,
			shouldClose: true,
			shouldAutoClose: false,
		});
	});

	it('resolves add transitions to hook-results state with contextual success', () => {
		expect(
			resolveMutationHookTransition({
				action: 'added',
				workspaceName: 'alpha',
				itemCount: 2,
				warnings: ['warning'],
				pendingHooks: [{ event: 'repo.add', repo: 'repo-a', hooks: ['post-checkout'] }],
				hookRuns: [{ event: 'repo.add', repo: 'repo-a', id: 'hook-1', status: 'running' }],
			}),
		).toEqual({
			phase: 'hook-results',
			hookResultContext: { action: 'added', name: 'alpha', itemCount: 2 },
			success: 'Added 2 items.',
			shouldClose: false,
			shouldAutoClose: false,
		});
	});

	it('resolves clean create hook activity with auto-close', () => {
		expect(
			resolveMutationHookTransition({
				action: 'created',
				workspaceName: 'alpha',
				warnings: [],
				pendingHooks: [],
				hookRuns: [{ event: 'workspace.create', repo: 'repo-a', id: 'hook-1', status: 'ok' }],
			}),
		).toEqual({
			phase: 'hook-results',
			hookResultContext: { action: 'created', name: 'alpha' },
			success: 'Created alpha.',
			shouldClose: false,
			shouldAutoClose: true,
		});
	});

	it('normalizes removal state and status refresh decisions', () => {
		expect(
			resolveRemovalState({
				removeDeleteFiles: false,
				removeForceDelete: true,
				removeConfirmText: 'DELETE',
				removeRepoConfirmRequired: false,
				removeRepoConfirmText: 'DELETE',
				removeRepoStatusRequested: true,
			}),
		).toEqual({
			removeForceDelete: false,
			removeConfirmText: '',
			removeRepoConfirmText: '',
			removeRepoStatusRequested: false,
		});

		expect(shouldRefreshRemoveRepoStatus(true, false)).toBe(true);
		expect(shouldRefreshRemoveRepoStatus(true, true)).toBe(false);
		expect(shouldRefreshRemoveRepoStatus(false, false)).toBe(false);
	});
});
