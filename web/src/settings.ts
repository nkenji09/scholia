import { useEffect, useState } from 'preact/hooks';

// Viewer-local display preferences (theme/font scale) — pure presentation,
// no bearing on the .pmem record model, so this lives outside the
// api.ts/types.ts data layer entirely. Persisted so it survives both
// `pmem view` and a `pmem export --html` reload, per the same localStorage
// approach comments (#18) uses.
//
// Density and accent-color used to be user-facing switches too (G-5 of
// .concierge/decision.md, "デザイン通り全部"), but were removed per user
// visual feedback (2026-07-11 tweaks2): the design mock itself never showed
// either control, so accent is now fixed to terracotta (tokens.css) and
// density to its default scale. See git history for the prior switchable
// versions if they're wanted back.

export type Theme = 'light' | 'dark';

export interface ViewerSettings {
  theme: Theme;
  fontScale: number;
}

const STORAGE_KEY = 'pmem-viewer-settings-v1';
const DEFAULTS: ViewerSettings = { theme: 'light', fontScale: 1 };
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

// data-theme goes on <html> (not #app) so tokens.css's :root-level custom
// properties — and native form-control theming via `color-scheme`, which
// only responds on the root element — both pick it up.
function apply(settings: ViewerSettings) {
  const root = document.documentElement;
  root.setAttribute('data-theme', settings.theme);
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
    incFont: () => setSettings((s) => ({ ...s, fontScale: Math.min(FONT_MAX, Math.round((s.fontScale + FONT_STEP) * 100) / 100) })),
    decFont: () => setSettings((s) => ({ ...s, fontScale: Math.max(FONT_MIN, Math.round((s.fontScale - FONT_STEP) * 100) / 100) })),
  };
}
