import { useEffect, useRef } from 'preact/hooks';
import { useT, useLang } from '../../i18n';
import type { ViewName } from '../../router';
import { useViewerSettings } from '../../settings';
import { useDrawer } from '../../drawer';
import { Icon } from '../shared/Icon';
import type { IconName } from '../shared/Icon';
import { useComments } from '../comments/useComments';
import { useLookups } from '../../lookups';
import { isStaticMode } from '../../api';

interface Props {
  view: ViewName;
  onSelectView: (v: ViewName) => void;
}

// Every screen that renders a BrowseRail (the off-canvas drawer on narrow
// viewports needs a toggle for these, and only these — design's own
// `showFilterToggle: isNarrow && isBrowse` where isBrowse = view is
// 'tags'/'specs'; ours additionally has 'browse'/'spec' as hash-compat
// aliases for the same BrowseView, and 'vocab' since VocabView adopted the
// same rail (2026-07-11 tweaks2 §4) after the design was written).
function usesRail(view: ViewName): boolean {
  return view === 'tags' || view === 'browse' || view === 'spec' || view === 'vocab';
}

export function Header({ view, onSelectView }: Props) {
  const t = useT();
  const { lang, toggleLang } = useLang();
  const { settings, toggleTheme, incFont, decFont } = useViewerSettings();
  const { comments, panelOpen, openPanel } = useComments();
  const { isNarrow, toggleDrawer } = useDrawer();
  const { productName, headerSubtitle } = useLookups();
  const headerRef = useRef<HTMLElement>(null);

  // Nav mirrors the design's segmented-pill control (概要/タグ/仕様 + icons),
  // extended with Vocab — a screen the design didn't mock but which still
  // needs a reachable nav slot (.concierge/decision.md §A-4). Order is
  // 概要/語彙/タグ/仕様 per user visual feedback (2026-07-11 tweaks2: 語彙 moved
  // between 概要 and タグ). Traceability/Compare were also in that "not mocked"
  // set but were dropped from the nav entirely per an earlier user request —
  // not in the design, so removed for now rather than left half-styled; git
  // history has the prior version if they come back. 'spec' (the legacy
  // per-tag-report hash) is deliberately NOT a nav entry: it renders the same
  // BrowseView as 'tags' with a different initial focus, so having both as
  // separate buttons would just be two nav items doing the same thing. Config
  // is not here either — the design treats settings as a standalone icon
  // button, not a nav tab (see the header switches cluster below).
  //
  // Built inside the component (not module scope) so it re-renders with the
  // active language — strings pulled from `t`, not a module-level `strings`.
  // 'compare'（評価コックピット・diff-viz §2）はサーバモード限定（`GET
  // /api/diff` を叩く・static export はバケた ref データを持たない）なので
  // isStaticMode の時だけナビから外す — graceful hide（ビュー自体も直接
  // #/compare で踏んだ場合に備え static メッセージを出す・CompareView.tsx）。
  const NAV: Array<[ViewName, string, IconName]> = [
    ['home', t.nav.home, 'layout-dashboard'],
    ['vocab', t.nav.vocab, 'book-open'],
    ['tags', t.nav.tags, 'tags'],
    ['browse', t.nav.specs, 'scroll-text'],
    ...(isStaticMode ? [] : ([['compare', t.nav.compare, 'git-compare']] as Array<[ViewName, string, IconName]>)),
  ];

  // Rail responsiveness (drawer's fixed `top`, sticky rail's `top`/height,
  // backdrop's `inset`) all need the header's actual rendered height —
  // design hardcodes a HEADER=56 constant, but our header can wrap onto a
  // second line at narrow widths (flex-wrap on .topbar) where 56px would be
  // wrong, so this measures the real value instead of assuming it.
  useEffect(() => {
    const el = headerRef.current;
    if (!el) return;
    const apply = () => document.documentElement.style.setProperty('--header-h', `${el.offsetHeight}px`);
    apply();
    const ro = new ResizeObserver(apply);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const showFilterToggle = isNarrow && usesRail(view);

  return (
    <header class="topbar" ref={headerRef}>
      {showFilterToggle && (
        <button type="button" class="topbar-filter-toggle" aria-label={t.header.filterToggle} onClick={toggleDrawer}>
          <Icon name="sliders-horizontal" size={15} />
          {t.header.filterToggle}
        </button>
      )}

      <div class="topbar-logo">
        <span class="topbar-logo-mark">
          <Icon name="box" size={19} />
        </span>
        <div class="topbar-logo-text">
          <span class="topbar-logo-title">{productName}</span>
          <span class="topbar-logo-subtitle">{headerSubtitle}</span>
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

      <div class="header-switches">
        <div class="font-scale" role="group" aria-label={t.header.fontScaleGroupLabel}>
          <button type="button" aria-label={t.header.fontDec} onClick={decFont}>
            <Icon name="minus" size={14} />
          </button>
          <span class="font-scale-pct">{Math.round(settings.fontScale * 100)}%</span>
          <button type="button" aria-label={t.header.fontInc} onClick={incFont}>
            <Icon name="plus" size={14} />
          </button>
        </div>
        <button type="button" class="topbar-icon-btn lang-toggle-btn" aria-label={t.header.langToggle} title={t.header.langToggle} onClick={toggleLang}>
          <Icon name="languages" size={17} />
          <span class="lang-toggle-code">{lang === 'ja' ? 'EN' : 'JA'}</span>
        </button>
        <button type="button" class="topbar-icon-btn" aria-label={t.header.themeToggle} onClick={toggleTheme}>
          <Icon name={settings.theme === 'dark' ? 'moon' : 'sun'} size={17} />
        </button>
        {comments.length > 0 && (
          <button
            type="button"
            class={'topbar-icon-btn comment-header-btn' + (panelOpen ? ' active' : '')}
            aria-label={t.header.commentList}
            onClick={openPanel}
          >
            <Icon name="message-filled" size={18} />
            <span class="comment-header-badge">{comments.length}</span>
          </button>
        )}
        <button
          type="button"
          class={'topbar-icon-btn' + (view === 'config' ? ' active' : '')}
          aria-label={t.nav.config}
          title={t.nav.config}
          onClick={() => onSelectView('config')}
        >
          <Icon name="settings" size={17} />
        </button>
      </div>
    </header>
  );
}
