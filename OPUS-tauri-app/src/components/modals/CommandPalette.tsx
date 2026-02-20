import { useState, useRef, useEffect, useMemo, useCallback } from 'react';
import { useAppStore } from '@/state/store';
import { getVisibleCommands } from '@/commands/registry';
import { fuzzyFilter } from '@/commands/fuzzyMatch';
import type { CommandDefinition, KeyboardShortcut } from '@/commands/registry';
import type { FuzzyResult } from '@/commands/fuzzyMatch';
import { Search } from 'lucide-react';
import './CommandPalette.css';

const KEY_SYMBOLS: Record<string, string> = {
  arrowup: '\u2191',
  arrowdown: '\u2193',
  arrowleft: '\u2190',
  arrowright: '\u2192',
  enter: '\u21A9',
  escape: 'Esc',
  backspace: '\u232B',
};

function formatShortcut(shortcut: KeyboardShortcut): string {
  const parts: string[] = [];
  if (shortcut.modifiers.includes('meta')) parts.push('\u2318');
  if (shortcut.modifiers.includes('ctrl')) parts.push('\u2303');
  if (shortcut.modifiers.includes('alt')) parts.push('\u2325');
  if (shortcut.modifiers.includes('shift')) parts.push('\u21E7');
  parts.push(KEY_SYMBOLS[shortcut.key] ?? shortcut.key.toUpperCase());
  return parts.join('');
}

function HighlightedLabel({ label, matches }: { label: string; matches: number[] }) {
  if (matches.length === 0) return <>{label}</>;
  const matchSet = new Set(matches);
  return (
    <>
      {label.split('').map((char, i) =>
        matchSet.has(i) ? (
          <span key={i} className="command-palette__match">{char}</span>
        ) : (
          <span key={i}>{char}</span>
        ),
      )}
    </>
  );
}

const CATEGORY_LABELS: Record<string, string> = {
  navigation: 'Navigation',
  workspace: 'Workspace',
  terminal: 'Terminal',
  workset: 'Workset',
  app: 'App',
};

export function CommandPalette() {
  const closeModal = useAppStore((s) => s.closeModal);
  const [query, setQuery] = useState('');
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const visibleCommands = useMemo(() => getVisibleCommands(), []);
  const results: FuzzyResult<CommandDefinition>[] = useMemo(
    () => fuzzyFilter(visibleCommands, query, (cmd) => cmd.label),
    [visibleCommands, query],
  );

  // Group by category, preserving a flat index for keyboard navigation
  const grouped = useMemo(() => {
    const flat = results.map((r, i) => ({ ...r, flatIndex: i }));
    const groups = new Map<string, typeof flat>();
    for (const r of flat) {
      const cat = r.item.category;
      if (!groups.has(cat)) groups.set(cat, []);
      groups.get(cat)!.push(r);
    }
    return groups;
  }, [results]);

  useEffect(() => {
    setSelectedIndex(0);
  }, [query]);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  useEffect(() => {
    const el = listRef.current?.querySelector('[data-selected="true"]');
    el?.scrollIntoView({ block: 'nearest' });
  }, [selectedIndex]);

  const executeSelected = useCallback(() => {
    const result = results[selectedIndex];
    if (result) {
      closeModal();
      requestAnimationFrame(() => result.item.execute());
    }
  }, [results, selectedIndex, closeModal]);

  function handleKeyDown(e: React.KeyboardEvent) {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex((i) => Math.min(i + 1, results.length - 1));
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex((i) => Math.max(i - 1, 0));
        break;
      case 'Enter':
        e.preventDefault();
        executeSelected();
        break;
      case 'Escape':
        e.preventDefault();
        closeModal();
        break;
    }
  }

  return (
    <div className="command-palette__overlay" onClick={closeModal}>
      <div className="command-palette" onClick={(e) => e.stopPropagation()}>
        <div className="command-palette__input-row">
          <Search size={16} className="command-palette__search-icon" />
          <input
            ref={inputRef}
            className="command-palette__input"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a command..."
            spellCheck={false}
            autoComplete="off"
          />
        </div>
        <div className="command-palette__list" ref={listRef}>
          {results.length === 0 && (
            <div className="command-palette__empty">No matching commands</div>
          )}
          {Array.from(grouped.entries()).map(([category, items]) => (
            <div key={category} className="command-palette__group">
              <div className="command-palette__group-label">
                {CATEGORY_LABELS[category] ?? category}
              </div>
              {items.map((r) => {
                const Icon = r.item.icon;
                return (
                  <button
                    key={r.item.id}
                    className={`command-palette__item ${
                      r.flatIndex === selectedIndex ? 'command-palette__item--selected' : ''
                    }`}
                    data-selected={r.flatIndex === selectedIndex}
                    onClick={() => {
                      closeModal();
                      requestAnimationFrame(() => r.item.execute());
                    }}
                    onMouseEnter={() => setSelectedIndex(r.flatIndex)}
                  >
                    {Icon && <Icon size={16} className="command-palette__item-icon" />}
                    <span className="command-palette__item-label">
                      <HighlightedLabel label={r.item.label} matches={r.matches} />
                    </span>
                    {r.item.shortcut && (
                      <kbd className="command-palette__kbd">
                        {formatShortcut(r.item.shortcut)}
                      </kbd>
                    )}
                  </button>
                );
              })}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
