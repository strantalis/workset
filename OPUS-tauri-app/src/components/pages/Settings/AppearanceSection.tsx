import { useAppStore } from '@/state/store';
import { themes } from '@/styles/themes';
import type { ThemeDefinition } from '@/styles/themes';
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

  const darkThemes = themes.filter((t) => t.group === 'dark');
  const lightThemes = themes.filter((t) => t.group === 'light');

  return (
    <section className="settings-section">
      <h3 className="settings-section__title">Appearance</h3>

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
  );
}
