import { useState } from 'react';
import type { DiffSummary, DiffFileSummary } from '@/types/diff';
import { DiffFileRow } from './DiffFileRow';
import { ChevronDown, ChevronRight, GitBranch } from 'lucide-react';
import './DiffRepoGroup.css';

type Props = {
  repo: string;
  summary: DiffSummary;
  onFileClick: (file: DiffFileSummary) => void;
};

export function DiffRepoGroup({ repo, summary, onFileClick }: Props) {
  const [expanded, setExpanded] = useState(true);

  return (
    <div className="diff-repo-group">
      <button className="diff-repo-group__header" onClick={() => setExpanded(!expanded)}>
        {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <GitBranch size={13} className="diff-repo-group__icon" />
        <span className="diff-repo-group__name">{repo}</span>
        <span className="diff-repo-group__count">{summary.files.length}</span>
        <span className="diff-repo-group__stats">
          {summary.total_added > 0 && (
            <span className="diff-repo-group__added">+{summary.total_added}</span>
          )}
          {summary.total_removed > 0 && (
            <span className="diff-repo-group__removed">-{summary.total_removed}</span>
          )}
        </span>
      </button>
      {expanded && (
        <div className="diff-repo-group__files">
          {summary.files.map((file) => (
            <DiffFileRow
              key={file.path}
              file={file}
              onClick={() => onFileClick(file)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
