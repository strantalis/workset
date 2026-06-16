import { useCallback, useEffect, useRef } from 'react';
import type { DiffFileSummary } from '@/types/diff';
import { useAppStore } from '@/state/store';
import { DiffRepoGroup } from './DiffRepoGroup';
import { useTauriEvent } from '@/hooks/useTauriEvent';
import { listWorkspaceRepos } from '@/api/repos';
import { diffWatchStart, diffWatchStop } from '@/api/diff';
import './DiffNavigator.css';

type DiffSummaryEvent = {
  workspace_name: string;
  repo: string;
  summary: { files: DiffFileSummary[]; total_added: number; total_removed: number };
};

export function DiffNavigator() {
  const repoDiffs = useAppStore((s) => s.repoDiffs);
  const updateDiffSummary = useAppStore((s) => s.updateDiffSummary);
  const loadDiffSummary = useAppStore((s) => s.loadDiffSummary);
  const activeWorkspaceName = useAppStore((s) => s.activeWorkspaceName);
  const focusedPaneId = useAppStore((s) => s.focusedPaneId);
  const addTab = useAppStore((s) => s.addTab);
  const watchedReposRef = useRef<{ workspace: string; repo: string }[]>([]);

  // Listen for watcher events
  const handleSummaryEvent = useCallback(
    (payload: DiffSummaryEvent) => {
      updateDiffSummary(payload.repo, payload.summary);
    },
    [updateDiffSummary],
  );
  useTauriEvent<DiffSummaryEvent>('diff:summary', handleSummaryEvent);

  // Fetch repos, load initial diffs, start watchers when workspace changes
  useEffect(() => {
    if (!activeWorkspaceName) return;
    let cancelled = false;
    const wsName = activeWorkspaceName;

    (async () => {
      try {
        const repos = await listWorkspaceRepos(wsName);
        if (cancelled) return;

        for (const repo of repos) {
          if (repo.missing) continue;
          // Load initial diff summary
          loadDiffSummary(wsName, repo.name, repo.worktree_path);
          // Start background watcher
          diffWatchStart(wsName, repo.name, repo.worktree_path).catch(() => {});
          watchedReposRef.current.push({ workspace: wsName, repo: repo.name });
        }
      } catch (err) {
        console.error('Failed to load workspace repos for diff:', err);
      }
    })();

    return () => {
      cancelled = true;
      // Stop watchers on cleanup
      for (const { workspace, repo } of watchedReposRef.current) {
        diffWatchStop(workspace, repo).catch(() => {});
      }
      watchedReposRef.current = [];
    };
  }, [activeWorkspaceName, loadDiffSummary]);

  const repos = Object.values(repoDiffs).filter(
    (rd) => rd.summary && rd.summary.files.length > 0,
  );

  function handleFileClick(repo: string, repoPath: string, file: DiffFileSummary) {
    const paneId = focusedPaneId ?? 'main';
    const tabId = `diff-${repo}-${file.path}`;
    addTab(paneId, {
      id: tabId,
      terminal_id: '',
      title: file.path,
      kind: 'diff',
      diff_repo: repo,
      diff_repo_path: repoPath,
      diff_file_path: file.path,
      diff_prev_path: file.prev_path,
      diff_status: file.status,
    });
  }

  if (repos.length === 0) {
    return (
      <div className="diff-navigator">
        <div className="diff-navigator__header">Diff</div>
        <div className="diff-navigator__empty">No changes detected</div>
      </div>
    );
  }

  const totalFiles = repos.reduce((sum, r) => sum + (r.summary?.files.length ?? 0), 0);
  const totalAdded = repos.reduce((sum, r) => sum + (r.summary?.total_added ?? 0), 0);
  const totalRemoved = repos.reduce((sum, r) => sum + (r.summary?.total_removed ?? 0), 0);

  return (
    <div className="diff-navigator">
      <div className="diff-navigator__header">
        <span>Diff</span>
        <span className="diff-navigator__totals">
          {totalFiles} file{totalFiles !== 1 ? 's' : ''}
          {totalAdded > 0 && <span className="diff-navigator__added"> +{totalAdded}</span>}
          {totalRemoved > 0 && <span className="diff-navigator__removed"> -{totalRemoved}</span>}
        </span>
      </div>
      <div className="diff-navigator__body">
        {repos.map((rd) => (
          <DiffRepoGroup
            key={rd.repo}
            repo={rd.repo}
            summary={rd.summary!}
            onFileClick={(file) => handleFileClick(rd.repo, rd.repoPath, file)}
          />
        ))}
      </div>
    </div>
  );
}
