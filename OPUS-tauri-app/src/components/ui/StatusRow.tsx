import { CheckCircle, XCircle, AlertCircle } from 'lucide-react';
import './StatusRow.css';

type Props = {
  label: string;
  status: 'ok' | 'warning' | 'error' | 'unknown';
  value?: string;
  action?: React.ReactNode;
};

const statusIcons = {
  ok: <CheckCircle size={14} className="status-row__icon--ok" />,
  warning: <AlertCircle size={14} className="status-row__icon--warning" />,
  error: <XCircle size={14} className="status-row__icon--error" />,
  unknown: <AlertCircle size={14} className="status-row__icon--unknown" />,
};

export function StatusRow({ label, status, value, action }: Props) {
  return (
    <div className="status-row">
      {statusIcons[status]}
      <span className="status-row__label">{label}</span>
      {value && <span className="status-row__value">{value}</span>}
      {action && <div className="status-row__action">{action}</div>}
    </div>
  );
}
