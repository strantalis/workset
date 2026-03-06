export type GitHubRepo = {
  full_name: string;
  description: string | null;
  private: boolean;
};

export type GitHubAuthStatus = {
  available: boolean;
  authenticated: boolean;
  message: string;
};

export type GitHubAccount = {
  login: string;
  active: boolean;
};
