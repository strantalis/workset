import { invoke } from './invoke';
import type { EnvSnapshot, SessiondStatus, CliStatus } from '@/types/diagnostics';

export function envSnapshot(): Promise<EnvSnapshot> {
  return invoke<EnvSnapshot>('diagnostics_env_snapshot', {});
}

export function reloadLoginEnv(): Promise<EnvSnapshot> {
  return invoke<EnvSnapshot>('diagnostics_reload_login_env', {});
}

export function sessiondStatus(): Promise<SessiondStatus> {
  return invoke<SessiondStatus>('diagnostics_sessiond_status', {});
}

export function sessiondRestart(): Promise<SessiondStatus> {
  return invoke<SessiondStatus>('diagnostics_sessiond_restart', {});
}

export function cliStatus(): Promise<CliStatus> {
  return invoke<CliStatus>('diagnostics_cli_status', {});
}
