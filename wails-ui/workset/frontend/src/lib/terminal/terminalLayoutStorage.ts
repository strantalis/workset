import { normalizeLayout, type TerminalLayout } from './terminalLayoutTree';

const MIGRATION_VERSION = 1;
const LEGACY_STORAGE_PREFIX = 'workset:terminal-layout:';
const MIGRATION_PREFIX = 'workset:terminal-layout:migrated:v';

const migrationKey = (id: string): string => `${MIGRATION_PREFIX}${MIGRATION_VERSION}:${id}`;
const legacyStorageKey = (id: string): string => `${LEGACY_STORAGE_PREFIX}${id}`;

export const shouldRunLayoutMigration = (id: string): boolean => {
	if (!id || typeof localStorage === 'undefined') return false;
	try {
		return localStorage.getItem(migrationKey(id)) !== '1';
	} catch {
		return false;
	}
};

export const markLayoutMigrationComplete = (id: string): void => {
	if (!id || typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(migrationKey(id), '1');
	} catch {
		// Ignore storage failures.
	}
};

export const clearLegacyTerminalLayout = (id: string): void => {
	if (!id || typeof localStorage === 'undefined') return;
	try {
		localStorage.removeItem(legacyStorageKey(id));
	} catch {
		// Ignore storage failures.
	}
};

export const loadLegacyTerminalLayout = (id: string): TerminalLayout | null => {
	if (!id || typeof localStorage === 'undefined') return null;
	try {
		const raw = localStorage.getItem(legacyStorageKey(id));
		if (!raw) return null;
		const parsed = JSON.parse(raw) as TerminalLayout;
		return normalizeLayout(parsed);
	} catch {
		return null;
	}
};
