import type { DiffFileSummary } from '@/types/diff';
import { FileText, FilePlus, FileMinus, FileEdit, ArrowRightLeft } from 'lucide-react';
import './DiffFileRow.css';

type Props = {
  file: DiffFileSummary;
  onClick: () => void;
};

const statusIcons: Record<string, typeof FileText> = {
  M: FileEdit,
  A: FilePlus,
  D: FileMinus,
  R: ArrowRightLeft,
};

export function DiffFileRow({ file, onClick }: Props) {
  const Icon = statusIcons[file.status] ?? FileText;
  const filename = file.path.split('/').pop() ?? file.path;
  const dir = file.path.includes('/') ? file.path.slice(0, file.path.lastIndexOf('/')) : '';

  return (
    <button className="diff-file-row" onClick={onClick}>
      <Icon size={13} className={`diff-file-row__icon diff-file-row__icon--${file.status.toLowerCase()}`} />
      <span className="diff-file-row__path">
        <span className="diff-file-row__name">{filename}</span>
        {dir && <span className="diff-file-row__dir">{dir}/</span>}
      </span>
      <span className="diff-file-row__stats">
        {file.added > 0 && <span className="diff-file-row__added">+{file.added}</span>}
        {file.removed > 0 && <span className="diff-file-row__removed">-{file.removed}</span>}
      </span>
    </button>
  );
}
