import { useState, useEffect } from 'react';
import { useAppStore } from '@/state/store';
import { Input } from '@/components/ui/Input';
import { Button } from '@/components/ui/Button';
import { StatusRow } from '@/components/ui/StatusRow';
import {
  envSnapshot,
  reloadLoginEnv,
  sessiondStatus,
  sessiondRestart,
  cliStatus,
} from '@/api/diagnostics';
import type { EnvSnapshot, SessiondStatus, CliStatus } from '@/types/diagnostics';
import './SettingsPage.css';

export function SettingsPage() {
  const section = useAppStore((s) => s.settingsSection);

  return (
    <div className="settings-page">
      <h2 className="settings-page__title">Settings</h2>
      {section === 'app' && <AppSettingsSection />}
      {section === 'workset' && <WorksetSettingsSection />}
      {section === 'diagnostics' && <DiagnosticsSection />}
    </div>
  );
}

// ---------------------------------------------------------------------------
// App Settings
// ---------------------------------------------------------------------------
function AppSettingsSection() {
  const [env, setEnv] = useState<EnvSnapshot | null>(null);

  useEffect(() => {
    envSnapshot().then(setEnv).catch(() => {});
  }, []);

  return (
    <section className="settings-section">
      <h3 className="settings-section__title">App Settings</h3>
      <div className="settings-grid">
        <div className="settings-field">
          <label className="settings-field__label">Default Shell</label>
          <Input value={env?.shell ?? ''} placeholder="/bin/zsh" disabled />
        </div>
        <div className="settings-field">
          <label className="settings-field__label">Agent Command</label>
          <Input placeholder="claude" disabled />
        </div>
      </div>
    </section>
  );
}

// ---------------------------------------------------------------------------
// Workset Settings
// ---------------------------------------------------------------------------
function WorksetSettingsSection() {
  const worksets = useAppStore((s) => s.worksets);
  const activeWorksetId = useAppStore((s) => s.activeWorksetId);
  const activeWorkset = worksets.find((w) => w.id === activeWorksetId);

  if (!activeWorkset) {
    return (
      <section className="settings-section">
        <h3 className="settings-section__title">Workset Settings</h3>
        <p className="settings-field__hint">No workset selected</p>
      </section>
    );
  }

  return (
    <section className="settings-section">
      <h3 className="settings-section__title">Workset: {activeWorkset.name}</h3>
      <div className="settings-grid">
        <div className="settings-field">
          <label className="settings-field__label">Repos</label>
          {activeWorkset.repos.length === 0 ? (
            <span className="settings-field__hint">No repositories added</span>
          ) : (
            <ul className="settings-repo-list">
              {activeWorkset.repos.map((r) => (
                <li key={r} className="settings-repo-list__item">
                  <code>{r}</code>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </section>
  );
}

// ---------------------------------------------------------------------------
// Diagnostics
// ---------------------------------------------------------------------------
function DiagnosticsSection() {
  const [env, setEnv] = useState<EnvSnapshot | null>(null);
  const [sessiond, setSessiond] = useState<SessiondStatus | null>(null);
  const [cli, setCli] = useState<CliStatus | null>(null);
  const [reloading, setReloading] = useState(false);
  const [restarting, setRestarting] = useState(false);

  useEffect(() => {
    envSnapshot().then(setEnv).catch(() => {});
    sessiondStatus().then(setSessiond).catch(() => {});
    cliStatus().then(setCli).catch(() => {});
  }, []);

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
      <section className="settings-section">
        <h3 className="settings-section__title">Diagnostics</h3>
        <div className="settings-diagnostics">
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
      </section>

      <section className="settings-section">
        <h3 className="settings-section__title">Environment Details</h3>
        {env && (
          <div className="settings-env-details">
            <div className="settings-env-row">
              <span className="settings-env-label">HOME</span>
              <code className="settings-env-value">{env.home}</code>
            </div>
            <div className="settings-env-row">
              <span className="settings-env-label">PATH</span>
              <code className="settings-env-value settings-env-value--wrap">{env.path}</code>
            </div>
            {env.git_ssh_command && (
              <div className="settings-env-row">
                <span className="settings-env-label">GIT_SSH_COMMAND</span>
                <code className="settings-env-value">{env.git_ssh_command}</code>
              </div>
            )}
          </div>
        )}
      </section>
    </>
  );
}
