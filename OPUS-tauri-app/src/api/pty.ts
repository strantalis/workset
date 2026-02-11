import { invoke } from './invoke';
import { Channel } from '@tauri-apps/api/core';
import type { TerminalLayout } from '@/types/layout';

// --- New terminal API (portable-pty based) ---

export type PtyEvent =
  | { type: 'Data'; data: string }
  | { type: 'Closed'; exit_code: number | null }
  | { type: 'Error'; message: string };

export function terminalSpawn(
  terminalId: string,
  cwd: string,
  onEvent: (e: PtyEvent) => void,
): Promise<void> {
  const channel = new Channel<PtyEvent>();
  channel.onmessage = onEvent;
  return invoke<void>('terminal_spawn', { terminalId, cwd, channel });
}

export function terminalAttach(
  terminalId: string,
  onEvent: (e: PtyEvent) => void,
): Promise<void> {
  const channel = new Channel<PtyEvent>();
  channel.onmessage = onEvent;
  return invoke<void>('terminal_attach', { terminalId, channel });
}

export function terminalDetach(terminalId: string): Promise<void> {
  return invoke<void>('terminal_detach', { terminalId });
}

export function terminalWrite(terminalId: string, data: string): Promise<void> {
  return invoke<void>('terminal_write', { terminalId, data });
}

export function terminalResize(terminalId: string, cols: number, rows: number): Promise<void> {
  return invoke<void>('terminal_resize', { terminalId, cols, rows });
}

export function terminalKill(terminalId: string): Promise<void> {
  return invoke<void>('terminal_kill', { terminalId });
}

// --- Layout persistence (unchanged) ---

export function layoutGet(workspaceName: string): Promise<TerminalLayout | null> {
  return invoke<TerminalLayout | null>('layout_get', { workspaceName });
}

export function layoutSave(workspaceName: string, layout: TerminalLayout): Promise<void> {
  return invoke<void>('layout_save', { workspaceName, layout });
}
