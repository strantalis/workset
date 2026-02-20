import { useState, useEffect, useRef, useMemo } from 'react';
import { useAppStore } from '@/state/store';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { listGitHubRepos, githubAuthStatus, listGitHubAccounts, switchGitHubAccount } from '@/api/github';
import type { GitHubRepo, GitHubAccount } from '@/types/github';
import { X, FolderGit2, Lock, Search, ChevronDown, Check, Loader2 } from 'lucide-react';
import './Modal.css';
import '../pages/CommandCenter/CommandCenterPage.css';

type Step = 'name' | 'repos';

function parseRepoName(repoUrl: string): { org: string; repo: string } {
  const match = repoUrl.match(/(?:github\.com\/)?([^/]+)\/([^/]+?)(?:\.git)?$/);
  if (match) return { org: match[1], repo: match[2] };
  return { org: '', repo: repoUrl };
}

export function WorksetCreateModal() {
  const createWorkset = useAppStore((s) => s.createWorkset);
  const setActiveWorkset = useAppStore((s) => s.setActiveWorkset);
  const addWorksetRepo = useAppStore((s) => s.addWorksetRepo);
  const closeModal = useAppStore((s) => s.closeModal);

  const [step, setStep] = useState<Step>('name');
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);

  // Repos step
  const [repoInput, setRepoInput] = useState('');
  const [repos, setRepos] = useState<string[]>([]);
  const [addingRepo, setAddingRepo] = useState(false);
  const [repoError, setRepoError] = useState<string | null>(null);

  // GitHub autocomplete state
  const [allRepos, setAllRepos] = useState<GitHubRepo[]>([]);
  const [ghAvailable, setGhAvailable] = useState<boolean | null>(null);
  const [reposLoading, setReposLoading] = useState(false);
  const [accounts, setAccounts] = useState<GitHubAccount[]>([]);
  const [activeAccount, setActiveAccount] = useState<string | null>(null);
  const [switching, setSwitching] = useState(false);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const [accountDropdownOpen, setAccountDropdownOpen] = useState(false);
  const searchRef = useRef<HTMLDivElement>(null);
  const accountDropdownRef = useRef<HTMLDivElement>(null);

  function loadRepos() {
    setReposLoading(true);
    listGitHubRepos()
      .then(setAllRepos)
      .catch(() => {})
      .finally(() => setReposLoading(false));
  }

  // Pre-fetch GitHub data on mount so it's ready when user reaches the repos step
  useEffect(() => {
    setReposLoading(true);
    githubAuthStatus()
      .then((s) => {
        setGhAvailable(s.authenticated);
        if (s.authenticated) {
          loadRepos();
          listGitHubAccounts()
            .then((accts) => {
              setAccounts(accts);
              const active = accts.find((a) => a.active);
              if (active) setActiveAccount(active.login);
            })
            .catch(() => {});
        } else {
          setReposLoading(false);
        }
      })
      .catch(() => {
        setGhAvailable(false);
        setReposLoading(false);
      });
  }, []);

  async function handleSwitchAccount(login: string) {
    if (login === activeAccount || switching) return;
    setSwitching(true);
    try {
      await switchGitHubAccount(login);
      setActiveAccount(login);
      setAccounts((prev) =>
        prev.map((a) => ({ ...a, active: a.login === login })),
      );
      setAllRepos([]);
      setRepoInput('');
      loadRepos();
    } finally {
      setSwitching(false);
    }
  }

  // Close dropdowns on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (searchRef.current && !searchRef.current.contains(e.target as Node)) {
        setShowSuggestions(false);
      }
      if (accountDropdownRef.current && !accountDropdownRef.current.contains(e.target as Node)) {
        setAccountDropdownOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  // Filter repos client-side — exclude already-added repos
  const filtered = useMemo(() => {
    const q = repoInput.trim().toLowerCase();
    if (!ghAvailable || q.length < 1 || allRepos.length === 0) return [];
    const added = new Set(repos.map((r) => {
      const { org, repo } = parseRepoName(r);
      return org ? `${org}/${repo}`.toLowerCase() : r.toLowerCase();
    }));
    return allRepos
      .filter((r) => {
        if (added.has(r.full_name.toLowerCase())) return false;
        return r.full_name.toLowerCase().includes(q);
      })
      .slice(0, 10);
  }, [repoInput, allRepos, ghAvailable, repos]);

  // Keep showSuggestions in sync with filtered results
  useEffect(() => {
    if (repoInput.trim().length >= 1 && filtered.length > 0) {
      setShowSuggestions(true);
    } else {
      setShowSuggestions(false);
    }
  }, [filtered, repoInput]);

  // Build description lookup
  const repoDescriptions = useMemo(() => {
    const map = new Map<string, string>();
    for (const r of allRepos) {
      if (r.description) map.set(r.full_name.toLowerCase(), r.description);
    }
    return map;
  }, [allRepos]);

  function handleInputChange(value: string) {
    setRepoInput(value);
    setSelectedIndex(-1);
    setRepoError(null);
  }

  async function handleCreateName() {
    if (!name.trim()) return;
    setLoading(true);
    try {
      const profile = await createWorkset(name.trim());
      // Don't await — let it run in background so the step transition is instant
      setActiveWorkset(profile.id);
      setStep('repos');
    } finally {
      setLoading(false);
    }
  }

  async function handleAddRepo(repoUrl?: string) {
    let source = repoUrl || repoInput.trim();
    if (!source) return;
    // Normalize short "org/repo" to SSH URL
    if (!source.includes('://') && !source.includes('@') && !source.startsWith('/') && !source.startsWith('~') && !source.startsWith('.') && source.includes('/')) {
      source = `git@github.com:${source}.git`;
    }
    setAddingRepo(true);
    setRepoError(null);
    setShowSuggestions(false);
    try {
      await addWorksetRepo(source);
      setRepos((prev) => [...prev, source]);
      setRepoInput('');
    } catch (err) {
      const msg =
        typeof err === 'object' && err !== null && 'message' in err
          ? String((err as { message: string }).message)
          : 'Failed to add repo';
      setRepoError(msg);
    } finally {
      setAddingRepo(false);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (!showSuggestions || filtered.length === 0) {
      if (e.key === 'Enter') {
        e.preventDefault();
        handleAddRepo();
      }
      return;
    }

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelectedIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (selectedIndex >= 0 && selectedIndex < filtered.length) {
        handleAddRepo(filtered[selectedIndex].full_name);
      } else {
        handleAddRepo();
      }
    } else if (e.key === 'Escape') {
      setShowSuggestions(false);
    }
  }

  function handleDone() {
    closeModal();
  }

  if (step === 'name') {
    return (
      <div className="modal-overlay" onClick={closeModal}>
        <div className="modal-repos-card" onClick={(e) => e.stopPropagation()}>
          <div className="modal-header">Create Workset</div>
          <div className="modal-body">
            <label className="modal-label">Name</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. payments-platform"
              autoFocus
              onKeyDown={(e) => e.key === 'Enter' && handleCreateName()}
            />
          </div>
          <div className="modal-footer">
            <Button variant="ghost" onClick={closeModal}>
              Cancel
            </Button>
            <Button
              variant="primary"
              onClick={handleCreateName}
              disabled={!name.trim() || loading}
            >
              {loading ? 'Creating...' : 'Next'}
            </Button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="modal-overlay" onClick={handleDone}>
      <div className="modal-repos-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">Add Repositories</div>
        <div className="modal-body">
          <div className="repo-search" ref={searchRef}>
            {accounts.length > 0 && (
              <div className="repo-search__account" ref={accountDropdownRef}>
                <button
                  className="repo-search__account-btn"
                  onClick={() => setAccountDropdownOpen((v) => !v)}
                  disabled={switching}
                >
                  <span className="repo-search__account-name">
                    {switching ? '...' : activeAccount ?? 'Account'}
                  </span>
                  <ChevronDown size={12} className="repo-search__account-chevron" />
                </button>
                {accountDropdownOpen && accounts.length > 1 && (
                  <div className="repo-search__account-dropdown">
                    {accounts.map((acct) => (
                      <button
                        key={acct.login}
                        className="repo-search__account-option"
                        onMouseDown={() => {
                          handleSwitchAccount(acct.login);
                          setAccountDropdownOpen(false);
                        }}
                      >
                        <span>{acct.login}</span>
                        {acct.login === activeAccount && <Check size={12} />}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}
            {accounts.length > 0 && <div className="repo-search__divider" />}
            {reposLoading ? (
              <Loader2 size={15} className="repo-search__icon repo-search__spinner" />
            ) : (
              <Search size={15} className="repo-search__icon" />
            )}
            <input
              className="repo-search__input"
              value={repoInput}
              onChange={(e) => handleInputChange(e.target.value)}
              onFocus={() => repoInput.trim().length >= 1 && filtered.length > 0 && setShowSuggestions(true)}
              onKeyDown={handleKeyDown}
              spellCheck={false}
              autoCorrect="off"
              autoCapitalize="off"
              autoFocus
              disabled={addingRepo}
              placeholder={
                reposLoading
                  ? 'Loading your repositories...'
                  : ghAvailable
                    ? 'Search your repos or paste a GitHub URL...'
                    : 'Paste org/repo or a full GitHub URL and press Enter'
              }
            />
            {addingRepo && <span className="repo-search__loading">Adding...</span>}
            {showSuggestions && filtered.length > 0 && (
              <div className="repo-suggestions">
                {filtered.map((repo, i) => {
                  const { org, repo: repoName } = parseRepoName(repo.full_name);
                  return (
                    <button
                      key={repo.full_name}
                      className={`repo-suggestions__item ${i === selectedIndex ? 'repo-suggestions__item--selected' : ''}`}
                      onMouseDown={() => handleAddRepo(repo.full_name)}
                      onMouseEnter={() => setSelectedIndex(i)}
                    >
                      <FolderGit2 size={14} className="repo-suggestions__icon" />
                      <div className="repo-suggestions__info">
                        <div className="repo-suggestions__name">
                          {org && <span className="repo-suggestions__org">{org}/</span>}
                          {repoName}
                        </div>
                        {repo.description && (
                          <div className="repo-suggestions__desc">{repo.description}</div>
                        )}
                      </div>
                      {repo.private && <Lock size={12} className="repo-suggestions__private" />}
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          {repoError && (
            <span style={{ fontSize: 12, color: 'var(--error)' }}>{repoError}</span>
          )}

          {repos.length > 0 && (
            <div className="repo-list">
              {repos.map((r) => {
                const { org, repo } = parseRepoName(r);
                const descKey = org ? `${org}/${repo}`.toLowerCase() : r.toLowerCase();
                const desc = repoDescriptions.get(descKey);
                return (
                  <div key={r} className="repo-row">
                    <FolderGit2 size={14} className="repo-row__icon" />
                    <div className="repo-row__info">
                      <div className="repo-row__name-line">
                        {org && <span className="repo-row__org">{org}/</span>}
                        <span className="repo-row__name">{repo}</span>
                      </div>
                      {desc && <div className="repo-row__desc">{desc}</div>}
                    </div>
                    <button
                      className="repo-row__remove"
                      style={{ opacity: 1 }}
                      onClick={() => setRepos((prev) => prev.filter((x) => x !== r))}
                      title="Remove"
                    >
                      <X size={12} />
                    </button>
                  </div>
                );
              })}
            </div>
          )}
        </div>
        <div className="modal-footer">
          <Button variant="ghost" onClick={handleDone}>
            {repos.length === 0 ? 'Skip' : 'Done'}
          </Button>
        </div>
      </div>
    </div>
  );
}
