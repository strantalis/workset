import './DiffRenderer.css';

type Props = {
  patch: string;
  truncated: boolean;
  binary?: boolean;
};

export function DiffRenderer({ patch, truncated, binary }: Props) {
  if (binary) {
    return (
      <div className="diff-renderer diff-renderer--binary">
        <span>Binary file — cannot display diff</span>
      </div>
    );
  }

  const lines = patch.split('\n');

  return (
    <div className="diff-renderer">
      {truncated && (
        <div className="diff-renderer__truncated">
          Diff truncated — file is too large to display in full
        </div>
      )}
      <pre className="diff-renderer__content">
        {lines.map((line, i) => {
          let cls = 'diff-line';
          if (line.startsWith('+') && !line.startsWith('+++')) {
            cls += ' diff-line--added';
          } else if (line.startsWith('-') && !line.startsWith('---')) {
            cls += ' diff-line--removed';
          } else if (line.startsWith('@@')) {
            cls += ' diff-line--hunk';
          } else if (line.startsWith('diff ') || line.startsWith('index ')) {
            cls += ' diff-line--meta';
          }
          return (
            <div key={i} className={cls}>
              <span className="diff-line__number">{i + 1}</span>
              <span className="diff-line__text">{line}</span>
            </div>
          );
        })}
      </pre>
    </div>
  );
}
