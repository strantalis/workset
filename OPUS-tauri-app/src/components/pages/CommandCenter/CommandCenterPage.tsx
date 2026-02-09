import { useState, useEffect, useRef, useMemo } from 'react';
import { useAppStore } from '@/state/store';
import { EmptyState } from '@/components/ui/EmptyState';
import { Button } from '@/components/ui/Button';
import { StatusRow } from '@/components/ui/StatusRow';
import { migrationStart } from '@/api/migrations';
import { listWorkspaces } from '@/api/workspaces';
import { listWorkspaceRepos } from '@/api/repos';
import { listGitHubRepos, githubAuthStatus, listGitHubAccounts, switchGitHubAccount } from '@/api/github';
import {
  envSnapshot,
  reloadLoginEnv,
  sessiondStatus,
  sessiondRestart,
  cliStatus,
} from '@/api/diagnostics';
import type { EnvSnapshot, SessiondStatus, CliStatus } from '@/types/diagnostics';
import type { GitHubRepo, GitHubAccount } from '@/types/github';
import type { RepoInstance } from '@/types/repo';
import { LayoutGrid, Layers, FolderGit2, Plus, Trash2, Lock, Search, ChevronDown, Check } from 'lucide-react';
import './CommandCenterPage.css';

export function CommandCenterPage() {
  const worksets = useAppStore((s) => s.worksets);
  const activeWorksetId = useAppStore((s) => s.activeWorksetId);
  const activeWorkspaceName = useAppStore((s) => s.activeWorkspaceName);
  const addWorksetRepo = useAppStore((s) => s.addWorksetRepo);
  const setActivePage = useAppStore((s) => s.setActivePage);
  const setCommandCenterSection = useAppStore((s) => s.setCommandCenterSection);
  const openModal = useAppStore((s) => s.openModal);
  const section = useAppStore((s) => s.commandCenterSection);

  const activeWorkset = worksets.find((w) => w.id === activeWorksetId);

  if (!activeWorkset) {
    return (
      <EmptyState
        icon={<LayoutGrid size={32} />}
        title="No workset selected"
        description="Select or create a workset to view its command center."
      />
    );
  }

  return (
    <div className="command-center">
      <div className="command-center__header">
        <h2>{activeWorkset.name}</h2>
      </div>
      {section === 'overview' && (
        <OverviewSection
          activeWorkset={activeWorkset}
          setActivePage={setActivePage}
          setCommandCenterSection={setCommandCenterSection}
        />
      )}
      {section === 'repositories' && (
        <RepositoriesSection
          activeWorkset={activeWorkset}
          activeWorksetId={activeWorksetId!}
          addWorksetRepo={addWorksetRepo}
          openModal={openModal}
        />
      )}
      {section === 'diagnostics' && (
        <DiagnosticsSection activeWorkspaceName={activeWorkspaceName} />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Overview
// ---------------------------------------------------------------------------
function OverviewSection({
  activeWorkset,
  setActivePage,
  setCommandCenterSection,
}: {
  activeWorkset: { name: string; repos: string[]; workspace_ids: string[] };
  setActivePage: (page: 'spaces') => void;
  setCommandCenterSection: (section: 'repositories') => void;
}) {
  return (
    <>
      <div className="command-center__stats">
        <div className="stat-card">
          <FolderGit2 size={18} className="stat-card__icon" />
          <div className="stat-card__value">{activeWorkset.repos.length}</div>
          <div className="stat-card__label">Repositories</div>
        </div>
        <div className="stat-card">
          <Layers size={18} className="stat-card__icon" />
          <div className="stat-card__value">{activeWorkset.workspace_ids.length}</div>
          <div className="stat-card__label">Workspaces</div>
        </div>
      </div>

      {activeWorkset.repos.length > 0 && (
        <div className="command-center__section">
          <h3 className="command-center__section-title">Repositories</h3>
          <div className="command-center__repo-list">
            {activeWorkset.repos.map((repo) => (
              <div key={repo} className="command-center__repo-item">
                <FolderGit2 size={14} className="command-center__repo-icon" />
                <span className="command-center__repo-name">{repo}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="command-center__actions">
        {activeWorkset.repos.length === 0 && (
          <Button variant="primary" onClick={() => setCommandCenterSection('repositories')}>
            <Plus size={14} /> Add Repositories
          </Button>
        )}
        <Button variant="secondary" onClick={() => setActivePage('spaces')}>
          Go to Spaces
        </Button>
      </div>
    </>
  );
}

// ---------------------------------------------------------------------------
// Repositories
// ---------------------------------------------------------------------------
function parseRepoName(repoUrl: string): { org: string; repo: string } {
  const match = repoUrl.match(/(?:github\.com\/)?([^/]+)\/([^/]+?)(?:\.git)?$/);
  if (match) return { org: match[1], repo: match[2] };
  return { org: '', repo: repoUrl };
}

function RepositoriesSection({
  activeWorkset,
  activeWorksetId,
  addWorksetRepo,
  openModal,
}: {
  activeWorkset: { repos: string[] };
  activeWorksetId: string;
  addWorksetRepo: (source: string) => Promise<void>;
  openModal: (type: string, props?: Record<string, unknown>) => void;
}) {
  const [newRepoUrl, setNewRepoUrl] = useState('');
  const [repoLoading, setRepoLoading] = useState(false);
  const [allRepos, setAllRepos] = useState<GitHubRepo[]>([]);
  const [ghAvailable, setGhAvailable] = useState<boolean | null>(null);
  const [accounts, setAccounts] = useState<GitHubAccount[]>([]);
  const [activeAccount, setActiveAccount] = useState<string | null>(null);
  const [switching, setSwitching] = useState(false);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const [accountDropdownOpen, setAccountDropdownOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const accountDropdownRef = useRef<HTMLDivElement>(null);

  function loadRepos() {
    listGitHubRepos()
      .then(setAllRepos)
      .catch(() => {});
  }

  // Check gh auth, load accounts and repos on mount
  useEffect(() => {
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
        }
      })
      .catch(() => setGhAvailable(false));
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
      setNewRepoUrl('');
      loadRepos();
    } finally {
      setSwitching(false);
    }
  }

  // Close dropdowns on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
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
    const q = newRepoUrl.trim().toLowerCase();
    if (!ghAvailable || q.length < 1 || allRepos.length === 0) return [];
    // Normalize stored URLs to short "org/repo" for comparison against full_name
    const added = new Set(activeWorkset.repos.map((r) => {
      const { org, repo } = parseRepoName(r);
      return org ? `${org}/${repo}`.toLowerCase() : r.toLowerCase();
    }));
    return allRepos
      .filter((r) => {
        if (added.has(r.full_name.toLowerCase())) return false;
        return r.full_name.toLowerCase().includes(q);
      })
      .slice(0, 15);
  }, [newRepoUrl, allRepos, ghAvailable, activeWorkset.repos]);

  function handleInputChange(value: string) {
    setNewRepoUrl(value);
    setSelectedIndex(-1);
    setShowSuggestions(value.trim().length >= 1 && filtered.length > 0);
  }

  // Keep showSuggestions in sync with filtered results
  useEffect(() => {
    if (newRepoUrl.trim().length >= 1 && filtered.length > 0) {
      setShowSuggestions(true);
    } else {
      setShowSuggestions(false);
    }
  }, [filtered, newRepoUrl]);

  // Build a lookup for descriptions from the fetched repo data
  // Keys are normalized "org/repo" so they match regardless of stored URL format
  const repoDescriptions = useMemo(() => {
    const map = new Map<string, string>();
    for (const r of allRepos) {
      if (r.description) map.set(r.full_name.toLowerCase(), r.description);
    }
    return map;
  }, [allRepos]);

  function selectSuggestion(fullName: string) {
    handleAddRepo(fullName);
  }

  async function handleAddRepo(repoUrl?: string) {
    let url = repoUrl || newRepoUrl.trim();
    if (!url || !activeWorksetId) return;
    // Normalize short "org/repo" to SSH URL so the Go backend recognizes it
    if (!url.includes('://') && !url.includes('@') && !url.startsWith('/') && !url.startsWith('~') && !url.startsWith('.') && url.includes('/')) {
      url = `git@github.com:${url}.git`;
    }
    setRepoLoading(true);
    setShowSuggestions(false);
    try {
      await addWorksetRepo(url);
      const freshWorkspaces = await listWorkspaces(activeWorksetId);
      const wsNames = freshWorkspaces.map((ws) => ws.name);
      if (wsNames.length > 0) {
        const { job_id } = await migrationStart({
          worksetId: activeWorksetId,
          repoUrl: url,
          action: 'add',
          workspaceNames: wsNames,
        });
        openModal('migration-status', {
          jobId: job_id,
          worksetId: activeWorksetId,
          repoUrl: url,
          action: 'add',
        });
      }
      setNewRepoUrl('');
    } finally {
      setRepoLoading(false);
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
        selectSuggestion(filtered[selectedIndex].full_name);
      } else {
        handleAddRepo();
      }
    } else if (e.key === 'Escape') {
      setShowSuggestions(false);
    }
  }

  function handleRemoveRepo(repo: string) {
    openModal('repo-remove-confirm', {
      worksetId: activeWorksetId,
      repoUrl: repo,
    });
  }

  return (
    <div className="command-center__section">
      <h3 className="command-center__section-title">Repositories</h3>

      <div className="repo-search" ref={containerRef}>
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
        <Search size={15} className="repo-search__icon" />
        <input
          className="repo-search__input"
          value={newRepoUrl}
          onChange={(e) => handleInputChange(e.target.value)}
          onFocus={() => newRepoUrl.trim().length >= 1 && filtered.length > 0 && setShowSuggestions(true)}
          onKeyDown={handleKeyDown}
          spellCheck={false}
          autoCorrect="off"
          autoCapitalize="off"
          disabled={repoLoading}
          placeholder={
            ghAvailable
              ? 'Search your repos or paste a GitHub URL...'
              : 'Paste org/repo or a full GitHub URL and press Enter'
          }
        />
        {repoLoading && <span className="repo-search__loading">Adding...</span>}
        {showSuggestions && filtered.length > 0 && (
          <div className="repo-suggestions">
            {filtered.map((repo, i) => {
              const { org, repo: name } = parseRepoName(repo.full_name);
              return (
                <button
                  key={repo.full_name}
                  className={`repo-suggestions__item ${i === selectedIndex ? 'repo-suggestions__item--selected' : ''}`}
                  onMouseDown={() => selectSuggestion(repo.full_name)}
                  onMouseEnter={() => setSelectedIndex(i)}
                >
                  <FolderGit2 size={14} className="repo-suggestions__icon" />
                  <div className="repo-suggestions__info">
                    <div className="repo-suggestions__name">
                      {org && <span className="repo-suggestions__org">{org}/</span>}
                      {name}
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

      {activeWorkset.repos.length > 0 ? (
        <div className="repo-list">
          {activeWorkset.repos.map((repoUrl) => {
            const { org, repo } = parseRepoName(repoUrl);
            const descKey = org ? `${org}/${repo}`.toLowerCase() : repoUrl.toLowerCase();
            const desc = repoDescriptions.get(descKey);
            return (
              <div key={repoUrl} className="repo-row">
                <FolderGit2 size={15} className="repo-row__icon" />
                <div className="repo-row__info">
                  <div className="repo-row__name-line">
                    {org && <span className="repo-row__org">{org}/</span>}
                    <span className="repo-row__name">{repo}</span>
                  </div>
                  {desc && <div className="repo-row__desc">{desc}</div>}
                </div>
                <button
                  className="repo-row__remove"
                  onClick={() => handleRemoveRepo(repoUrl)}
                  title="Remove repository"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            );
          })}
        </div>
      ) : (
        <EmptyState
          icon={<FolderGit2 size={36} />}
          title="No repositories yet"
          description="Search for a repo above or paste a GitHub URL and press Enter."
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Diagnostics
// ---------------------------------------------------------------------------
function DiagnosticsSection({ activeWorkspaceName }: { activeWorkspaceName: string | null }) {
  const [env, setEnv] = useState<EnvSnapshot | null>(null);
  const [sessiond, setSessiond] = useState<SessiondStatus | null>(null);
  const [cli, setCli] = useState<CliStatus | null>(null);
  const [repoInstances, setRepoInstances] = useState<RepoInstance[]>([]);
  const [reloading, setReloading] = useState(false);
  const [restarting, setRestarting] = useState(false);

  useEffect(() => {
    envSnapshot().then(setEnv).catch(() => {});
    sessiondStatus().then(setSessiond).catch(() => {});
    cliStatus().then(setCli).catch(() => {});
  }, []);

  useEffect(() => {
    if (activeWorkspaceName) {
      listWorkspaceRepos(activeWorkspaceName)
        .then(setRepoInstances)
        .catch(() => setRepoInstances([]));
    } else {
      setRepoInstances([]);
    }
  }, [activeWorkspaceName]);

  async function handleReloadEnv() {
    setReloading(true);
    try {
      const snap = await reloadLoginEnv();
      setEnv(snap);
    } finally {
      setReloading(false);
    }
  }

  async function handleRestartSessiond() {
    setRestarting(true);
    try {
      const status = await sessiondRestart();
      setSessiond(status);
    } finally {
      setRestarting(false);
    }
  }

  return (
    <>
      <div className="command-center__section">
        <h3 className="command-center__section-title">Runtime</h3>
        <div className="command-center__diagnostics">
          <StatusRow
            label="workset CLI"
            status={cli?.available ? 'ok' : 'error'}
            value={cli?.available ? cli.path : cli?.error ?? 'Not found'}
          />
          <StatusRow
            label="sessiond"
            status={sessiond?.running ? 'ok' : 'error'}
            value={sessiond?.running ? 'Running' : 'Stopped'}
            action={
              <Button
                variant="ghost"
                size="small"
                onClick={handleRestartSessiond}
                disabled={restarting}
              >
                {restarting ? 'Restarting...' : 'Restart'}
              </Button>
            }
          />
        </div>
      </div>

      <div className="command-center__section">
        <h3 className="command-center__section-title">Authentication</h3>
        <div className="command-center__diagnostics">
          <StatusRow
            label="SSH Agent"
            status={env?.ssh_auth_sock ? 'ok' : 'warning'}
            value={env?.ssh_auth_sock ? 'Connected' : 'Not set'}
          />
          <StatusRow
            label="GitHub Auth"
            status={env?.gh_auth_summary ? 'ok' : 'warning'}
            value={env?.gh_auth_summary ?? 'Not configured'}
          />
        </div>
      </div>

      <div className="command-center__section">
        <h3 className="command-center__section-title">Environment</h3>
        <div className="command-center__diagnostics">
          <StatusRow
            label="Login Environment"
            status={env ? 'ok' : 'unknown'}
            value={env?.shell ?? 'Unknown'}
            action={
              <Button
                variant="ghost"
                size="small"
                onClick={handleReloadEnv}
                disabled={reloading}
              >
                {reloading ? 'Reloading...' : 'Reload'}
              </Button>
            }
          />
        </div>
        {env && (
          <div className="command-center__env-details">
            <div className="command-center__env-row">
              <span className="command-center__env-label">HOME</span>
              <code className="command-center__env-value">{env.home}</code>
            </div>
            <div className="command-center__env-row">
              <span className="command-center__env-label">PATH</span>
              <code className="command-center__env-value command-center__env-value--wrap">{env.path}</code>
            </div>
            {env.git_ssh_command && (
              <div className="command-center__env-row">
                <span className="command-center__env-label">GIT_SSH_COMMAND</span>
                <code className="command-center__env-value">{env.git_ssh_command}</code>
              </div>
            )}
          </div>
        )}
      </div>

      {activeWorkspaceName && repoInstances.length > 0 && (
        <div className="command-center__section">
          <h3 className="command-center__section-title">Workspace Repos</h3>
          <div className="command-center__diagnostics">
            {repoInstances.map((repo) => (
              <StatusRow
                key={repo.name}
                label={repo.name}
                status={repo.missing ? 'error' : 'ok'}
                value={repo.missing ? 'Missing' : repo.worktree_path}
              />
            ))}
          </div>
        </div>
      )}
    </>
  );
}
