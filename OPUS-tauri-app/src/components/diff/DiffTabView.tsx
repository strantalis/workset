import { useEffect, useState } from 'react';
import { useAppStore } from '@/state/store';
import { DiffRenderer } from './DiffRenderer';
import type { FilePatch } from '@/types/diff';

type Props = {
  repoPath: string;
  filePath: string;
  prevPath?: string;
  status: string;
};

export function DiffTabView({ repoPath, filePath, prevPath, status }: Props) {
  const fetchFilePatch = useAppStore((s) => s.fetchFilePatch);
  const [patch, setPatch] = useState<FilePatch | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    fetchFilePatch(repoPath, filePath, prevPath, status)
      .then((result) => {
        if (!cancelled) setPatch(result);
      })
      .catch((err) => {
        if (!cancelled) {
          const msg = typeof err === 'object' && err !== null && 'message' in err
            ? String(err.message)
            : 'Failed to load diff';
          setError(msg);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => { cancelled = true; };
  }, [repoPath, filePath, prevPath, status, fetchFilePatch]);

  if (loading) {
    return (
      <div className="diff-tab-view diff-tab-view--loading">
        <span>Loading diff...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="diff-tab-view diff-tab-view--error">
        <span>{error}</span>
      </div>
    );
  }

  if (!patch || !patch.patch) {
    return (
      <div className="diff-tab-view diff-tab-view--empty">
        <span>No changes</span>
      </div>
    );
  }

  return (
    <DiffRenderer
      patch={patch.patch}
      truncated={patch.truncated}
      binary={patch.binary}
    />
  );
}
