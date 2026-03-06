import type { InputHTMLAttributes } from 'react';
import './Input.css';

type Props = InputHTMLAttributes<HTMLInputElement>;

export function Input({ className = '', ...rest }: Props) {
  return <input className={`input ${className}`} {...rest} />;
}
