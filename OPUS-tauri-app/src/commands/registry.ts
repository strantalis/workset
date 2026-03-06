import type { LucideIcon } from 'lucide-react';

export type CommandCategory =
  | 'navigation'
  | 'workspace'
  | 'terminal'
  | 'workset'
  | 'app';

export type KeyboardShortcut = {
  modifiers: ('meta' | 'shift' | 'alt' | 'ctrl')[];
  key: string;
};

export type CommandDefinition = {
  id: string;
  label: string;
  category: CommandCategory;
  icon?: LucideIcon;
  shortcut?: KeyboardShortcut;
  /** Return false to hide from palette in current context */
  when?: () => boolean;
  execute: () => void | Promise<void>;
};

const commands = new Map<string, CommandDefinition>();

export function registerCommand(cmd: CommandDefinition): () => void {
  commands.set(cmd.id, cmd);
  return () => {
    commands.delete(cmd.id);
  };
}

export function registerCommands(cmds: CommandDefinition[]): () => void {
  cmds.forEach((cmd) => commands.set(cmd.id, cmd));
  return () => {
    cmds.forEach((cmd) => commands.delete(cmd.id));
  };
}

export function getCommands(): CommandDefinition[] {
  return Array.from(commands.values());
}

export function getVisibleCommands(): CommandDefinition[] {
  return Array.from(commands.values()).filter(
    (cmd) => !cmd.when || cmd.when(),
  );
}
