import { invoke } from './invoke';
import type { PtyCreateResult, BootstrapPayload } from '@/types/pty';
import type { TerminalLayout } from '@/types/layout';

export function ptyCreate(): Promise<PtyCreateResult> {
  return invoke<PtyCreateResult>('pty_create', {});
}

export function ptyStart(
  workspaceName: string,
  terminalId: string,
  kind: 'terminal' | 'agent',
  cwd: string,
): Promise<void> {
  return invoke<void>('pty_start', { workspaceName, terminalId, kind, cwd });
}

export function ptyWrite(workspaceName: string, terminalId: string, data: string): Promise<void> {
  return invoke<void>('pty_write', { workspaceName, terminalId, data });
}

export function ptyResize(
  workspaceName: string,
  terminalId: string,
  cols: number,
  rows: number,
): Promise<void> {
  return invoke<void>('pty_resize', { workspaceName, terminalId, cols, rows });
}

export function ptyAck(workspaceName: string, terminalId: string, bytes: number): Promise<void> {
  return invoke<void>('pty_ack', { workspaceName, terminalId, bytes });
}

export function ptyBootstrap(
  workspaceName: string,
  terminalId: string,
): Promise<BootstrapPayload> {
  return invoke<BootstrapPayload>('pty_bootstrap', { workspaceName, terminalId });
}

export function ptyStop(workspaceName: string, terminalId: string): Promise<void> {
  return invoke<void>('pty_stop', { workspaceName, terminalId });
}

export function layoutGet(workspaceName: string): Promise<TerminalLayout | null> {
  return invoke<TerminalLayout | null>('layout_get', { workspaceName });
}

export function layoutSave(workspaceName: string, layout: TerminalLayout): Promise<void> {
  return invoke<void>('layout_save', { workspaceName, layout });
}
