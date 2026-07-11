import { strings } from '../../strings';
import type { ViewName } from '../../router';
import { ACCENTS, useViewerSettings } from '../../settings';
import { SearchBox } from '../SearchBox';
import { Icon } from '../shared/Icon';
import type { IconName } from '../shared/Icon';
import { useComments } from '../comments/useComments';

interface Props {
  view: ViewName;
  onSelectView: (v: ViewName) => void;
  onSelectTx: (id: string) => void;
}

// Nav mirrors the design's segmented-pill control (概要/タグ/仕様 + icons),
// extended with Vocab — a screen the design didn't mock but which still
// needs a reachable nav slot (.concierge/decision.md §A-4). Traceability/
// Compare were also in that "not mocked" set but were dropped from the nav
// entirely per a later user request (2026-07-11) — not in the design, so
// removed for now rather than left half-styled; git history has the prior
// version if they come back. 'spec' (the legacy per-tag-report hash) is
// deliberately NOT a nav entry: it renders the same BrowseView as 'tags'
// with a different initial focus, so having both as separate buttons would
// just be two nav items doing the same thing. Config is not here either —
// the design treats settings as a standalone icon button, not a nav tab
// (see the header switches cluster below).
const NAV: Array<[ViewName, string, IconName]> = [
  ['home', strings.nav.home, 'layout-dashboard'],
  ['tags', strings.nav.tags, 'tags'],
  ['browse', strings.nav.specs, 'scroll-text'],
  ['vocab', strings.nav.vocab, 'book-open'],
];

export function Header({ view, onSelectView, onSelectTx }: Props) {
  const { settings, toggleTheme, setDensity, setAccent, incFont, decFont } = useViewerSettings();
  const { comments, panelOpen, openPanel } = useComments();

  return (
    <header class="topbar">
      <div class="topbar-logo">
        <span class="topbar-logo-mark">
          <Icon name="box" size={19} />
        </span>
        <div class="topbar-logo-text">
          <span class="topbar-logo-title">pmem</span>
          <span class="topbar-logo-subtitle">product-memory</span>
        </div>
      </div>

      <nav class="topbar-nav">
        {NAV.map(([key, label, icon]) => {
          // 'spec' (legacy per-tag hash, kept for bookmark compat) renders
          // the same BrowseView 'tags' facet does — highlight タグ for it
          // too rather than leaving no tab active.
          const active = view === key || (key === 'tags' && view === 'spec');
          return (
            <button key={key} type="button" class={'topbar-nav-btn' + (active ? ' active' : '')} onClick={() => onSelectView(key)}>
              <Icon name={icon} size={16} />
              <span>{label}</span>
            </button>
          );
        })}
      </nav>

      <SearchBox onSelectTx={onSelectTx} />

      <div class="header-switches">
        <div class="font-scale" role="group" aria-label="文字サイズ">
          <button type="button" aria-label={strings.header.fontDec} onClick={decFont}>
            <Icon name="minus" size={14} />
          </button>
          <span class="font-scale-pct">{Math.round(settings.fontScale * 100)}%</span>
          <button type="button" aria-label={strings.header.fontInc} onClick={incFont}>
            <Icon name="plus" size={14} />
          </button>
        </div>
        <label class="header-select" title={strings.header.accent}>
          <span class="accent-dot" style={{ background: 'var(--lm-accent)' }} />
          <select value={settings.accent} onChange={(e) => setAccent((e.target as HTMLSelectElement).value as typeof settings.accent)}>
            {ACCENTS.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        </label>
        <label class="header-select" title="密度">
          <Icon name="sliders-horizontal" size={13} class="dim" />
          <select value={settings.density} onChange={(e) => setDensity((e.target as HTMLSelectElement).value as typeof settings.density)}>
            <option value="compact">{strings.header.density.compact}</option>
            <option value="normal">{strings.header.density.normal}</option>
            <option value="comfortable">{strings.header.density.comfortable}</option>
          </select>
        </label>
        <button type="button" class="topbar-icon-btn" aria-label={strings.header.themeToggle} onClick={toggleTheme}>
          <Icon name={settings.theme === 'dark' ? 'moon' : 'sun'} size={17} />
        </button>
        {comments.length > 0 && (
          <button type="button" class={'topbar-icon-btn comment-header-btn' + (panelOpen ? ' active' : '')} aria-label="コメント一覧" onClick={openPanel}>
            <Icon name="message-filled" size={17} />
            <span class="comment-header-badge">{comments.length}</span>
          </button>
        )}
        <button
          type="button"
          class={'topbar-icon-btn' + (view === 'config' ? ' active' : '')}
          aria-label={strings.nav.config}
          title={strings.nav.config}
          onClick={() => onSelectView('config')}
        >
          <Icon name="settings" size={17} />
        </button>
      </div>
    </header>
  );
}
