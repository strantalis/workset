import type { ButtonHTMLAttributes, ReactNode } from 'react';
import './Button.css';

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
  size?: 'sm' | 'md';
  children: ReactNode;
};

export function Button({ variant = 'secondary', size = 'md', className = '', children, ...rest }: Props) {
  return (
    <button className={`btn btn--${variant} btn--${size} ${className}`} {...rest}>
      {children}
    </button>
  );
}
