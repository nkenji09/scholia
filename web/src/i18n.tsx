import { createContext } from 'preact';
import type { ComponentChildren } from 'preact';
import { useContext, useEffect, useState } from 'preact/hooks';
import { DICTS } from './strings';
import type { Lang, Strings } from './strings';

export type { Lang, Strings };

// UI-chrome language switch — mirrors settings.ts's theme/font-scale
// persistence pattern exactly (localStorage key, load-once default, no
// browser-language auto-detect per user instruction: explicit toggle only).
// Default is Japanese when unset. Works identically under `pmem view`
// (server) and a `pmem export --html` file:// export since it never touches
// the network — same reasoning as settings.ts's own doc comment.

const STORAGE_KEY = 'pmem-lang';
const DEFAULT_LANG: Lang = 'ja';

function isLang(v: unknown): v is Lang {
  return v === 'ja' || v === 'en';
}

// Exported (not just used internally) so api.ts — a plain module with no
// component tree, so it can't call useLang()/useT() — can read the current
// language directly for its own chrome error strings.
export function loadLang(): Lang {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return isLang(raw) ? raw : DEFAULT_LANG;
  } catch {
    return DEFAULT_LANG;
  }
}

interface LangValue {
  lang: Lang;
  toggleLang: () => void;
}

const LangContext = createContext<LangValue | null>(null);

export function LangProvider({ children }: { children: ComponentChildren }) {
  const [lang, setLang] = useState<Lang>(loadLang);

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, lang);
    } catch {
      // Private-mode/quota failures — language still applies for this
      // session, just doesn't persist across reload.
    }
  }, [lang]);

  const value: LangValue = {
    lang,
    toggleLang: () => setLang((l) => (l === 'ja' ? 'en' : 'ja')),
  };
  return <LangContext.Provider value={value}>{children}</LangContext.Provider>;
}

export function useLang(): LangValue {
  const ctx = useContext(LangContext);
  if (!ctx) throw new Error('useLang() must be called within a LangProvider');
  return ctx;
}

/** The active language's translation table — components read chrome copy as `t.category.key` (replaces the old static `strings.category.key` import). */
export function useT(): Strings {
  const { lang } = useLang();
  return DICTS[lang];
}
