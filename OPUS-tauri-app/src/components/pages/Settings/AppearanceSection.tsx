import { useAppStore } from '@/state/store';
import { themes } from '@/styles/themes';
import type { ThemeDefinition } from '@/styles/themes';
import { Input } from '@/components/ui/Input';
import './AppearanceSection.css';

function ThemeCard({ theme, active, onSelect }: {
  theme: ThemeDefinition;
  active: boolean;
  onSelect: () => void;
}) {
  const t = theme.tokens;
  return (
    <button
      className={`theme-card ${active ? 'theme-card--active' : ''}`}
      onClick={onSelect}
    >
      <div
        className="theme-card__preview"
        style={{ background: t['--bg'] }}
      >
        <div className="theme-card__preview-panel" style={{ background: t['--panel'] }}>
          <div className="theme-card__preview-line" style={{ background: t['--text'], opacity: 0.7 }} />
          <div className="theme-card__preview-line theme-card__preview-line--short" style={{ background: t['--muted'], opacity: 0.5 }} />
        </div>
        <div className="theme-card__preview-accent" style={{ background: t['--accent'] }} />
      </div>
      <div className="theme-card__label">{theme.name}</div>
    </button>
  );
}

export function AppearanceSection() {
  const activeThemeId = useAppStore((s) => s.activeThemeId);
  const setTheme = useAppStore((s) => s.setTheme);
  const terminalStyle = useAppStore((s) => s.terminalStyle);
  const setTerminalStyle = useAppStore((s) => s.setTerminalStyle);

  const darkThemes = themes.filter((t) => t.group === 'dark');
  const lightThemes = themes.filter((t) => t.group === 'light');

  return (
    <>
      <section className="settings-section">
        <h3 className="settings-section__title">Theme</h3>

        <div className="appearance-group">
          <div className="appearance-group__label">Dark</div>
          <div className="appearance-grid">
            {darkThemes.map((t) => (
              <ThemeCard
                key={t.id}
                theme={t}
                active={activeThemeId === t.id}
                onSelect={() => setTheme(t.id)}
              />
            ))}
          </div>
        </div>

        <div className="appearance-group">
          <div className="appearance-group__label">Light</div>
          <div className="appearance-grid">
            {lightThemes.map((t) => (
              <ThemeCard
                key={t.id}
                theme={t}
                active={activeThemeId === t.id}
                onSelect={() => setTheme(t.id)}
              />
            ))}
          </div>
        </div>
      </section>

      <section className="settings-section">
        <h3 className="settings-section__title">Terminal</h3>

        <div className="settings-grid">
          <div className="settings-field">
            <label className="settings-field__label">Font Family</label>
            <Input
              value={terminalStyle.fontFamily}
              onChange={(e) => setTerminalStyle({ fontFamily: e.target.value })}
              placeholder="'JetBrains Mono', monospace"
            />
          </div>

          <div className="settings-field">
            <label className="settings-field__label">Font Size</label>
            <Input
              type="number"
              min={8}
              max={32}
              value={terminalStyle.fontSize}
              onChange={(e) => {
                const v = parseInt(e.target.value, 10);
                if (!isNaN(v) && v >= 8 && v <= 32) setTerminalStyle({ fontSize: v });
              }}
            />
          </div>

          <div className="settings-field">
            <label className="settings-field__label">Line Height</label>
            <Input
              type="number"
              min={1.0}
              max={2.0}
              step={0.1}
              value={terminalStyle.lineHeight}
              onChange={(e) => {
                const v = parseFloat(e.target.value);
                if (!isNaN(v) && v >= 1.0 && v <= 2.0) setTerminalStyle({ lineHeight: v });
              }}
            />
          </div>

          <div className="settings-field">
            <label className="settings-field__label">Cursor Blink</label>
            <label className="terminal-toggle">
              <input
                type="checkbox"
                checked={terminalStyle.cursorBlink}
                onChange={(e) => setTerminalStyle({ cursorBlink: e.target.checked })}
              />
              <span className="terminal-toggle__label">
                {terminalStyle.cursorBlink ? 'On' : 'Off'}
              </span>
            </label>
          </div>
        </div>
      </section>
    </>
  );
}
