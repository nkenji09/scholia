import { useEffect, useState } from 'preact/hooks';

// Viewer-local display preferences (theme/density/accent/font scale) — pure
// presentation, no bearing on the .pmem record model, so this lives outside
// the api.ts/types.ts data layer entirely (G-5 of .concierge/decision.md:
// "デザイン通り全部" — all four switches, persisted so they survive both
// `pmem view` and a `pmem export --html` reload, per the same localStorage
// approach comments (#18) uses).

export type Theme = 'light' | 'dark';
export type Density = 'compact' | 'normal' | 'comfortable';
export type Accent = 'indigo' | 'violet' | 'blue' | 'terracotta' | 'teal';

export interface ViewerSettings {
  theme: Theme;
  density: Density;
  accent: Accent;
  fontScale: number;
}

export const ACCENTS: Accent[] = ['indigo', 'violet', 'blue', 'terracotta', 'teal'];

const STORAGE_KEY = 'pmem-viewer-settings-v1';
const DEFAULTS: ViewerSettings = { theme: 'light', density: 'normal', accent: 'indigo', fontScale: 1 };
const FONT_MIN = 0.85;
const FONT_MAX = 1.5;
const FONT_STEP = 0.1;

function load(): ViewerSettings {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULTS;
    return { ...DEFAULTS, ...JSON.parse(raw) };
  } catch {
    return DEFAULTS;
  }
}

// data-theme/data-accent go on <html> (not #app) so tokens.css's :root-level
// custom properties — and native form-control theming via `color-scheme`,
// which only responds on the root element — both pick them up.
function apply(settings: ViewerSettings) {
  const root = document.documentElement;
  root.setAttribute('data-theme', settings.theme);
  root.setAttribute('data-accent', settings.accent);
  if (settings.density === 'normal') root.removeAttribute('data-density');
  else root.setAttribute('data-density', settings.density);
  root.style.setProperty('--lm-fs', String(settings.fontScale));
}

export function useViewerSettings() {
  const [settings, setSettings] = useState<ViewerSettings>(load);

  useEffect(() => {
    apply(settings);
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
    } catch {
      // Private-mode/quota failures still apply the setting for this
      // session (apply() above already ran) — just don't persist it.
    }
  }, [settings]);

  return {
    settings,
    toggleTheme: () => setSettings((s) => ({ ...s, theme: s.theme === 'dark' ? 'light' : 'dark' })),
    setDensity: (density: Density) => setSettings((s) => ({ ...s, density })),
    setAccent: (accent: Accent) => setSettings((s) => ({ ...s, accent })),
    incFont: () => setSettings((s) => ({ ...s, fontScale: Math.min(FONT_MAX, Math.round((s.fontScale + FONT_STEP) * 100) / 100) })),
    decFont: () => setSettings((s) => ({ ...s, fontScale: Math.max(FONT_MIN, Math.round((s.fontScale - FONT_STEP) * 100) / 100) })),
  };
}
