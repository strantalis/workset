export type ErrorCategory = 'auth' | 'network' | 'git' | 'config' | 'runtime' | 'unknown';

export type ErrorEnvelope = {
  category: ErrorCategory;
  operation: string;
  message: string;
  details?: string;
  retryable: boolean;
  suggested_actions: string[];
};
