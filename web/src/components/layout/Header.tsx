import { useEffect, useRef } from 'preact/hooks';
import { useT, useLang } from '../../i18n';
import type { ViewName } from '../../router';
import { useViewerSettings } from '../../settings';
import { useDrawer } from '../../drawer';
import { Icon } from '../shared/Icon';
import type { IconName } from '../shared/Icon';
import { useComments } from '../comments/useComments';
import { useLookups } from '../../lookups';

interface Props {
  view: ViewName;
  onSelectView: (v: ViewName) => void;
  /** True when the current view renders a BrowseRail — the off-canvas drawer
      on narrow viewports needs the 絞り込み toggle for these (design's own
      `showFilterToggle: isNarrow && isBrowse`). Computed by App
      (railActiveFor) since it depends on the full route: tags/browse/spec/
      vocab (BrowseView), vocab, decisions, and #/flow's index only — the
      per-action flow diagram has no rail (viewer-search-consistency). */
  railActive: boolean;
}

export function Header({ view, onSelectView, railActive }: Props) {
  const t = useT();
  const { lang, toggleLang } = useLang();
  const { settings, toggleTheme, incFont, decFont } = useViewerSettings();
  const { comments, panelOpen, openPanel } = useComments();
  const { isNarrow, toggleDrawer } = useDrawer();
  const { productName, headerSubtitle } = useLookups();
  const headerRef = useRef<HTMLElement>(null);

  // IA-rework (viewer-overview-browser): the nav collapses to just two tabs —
  // 概要 (the structure tree + component spec sheet) and ブラウザ (the unified
  // search/facet/detail surface). Every screen the old nav exposed as its own
  // tab (タグ/語彙/仕様/フロー/意思決定) is NOT deleted — it stays reachable as an
  // internal lens or detail route of these two (概要's sheet embeds coverage/
  // current-rules; ブラウザ's detail embeds vocab/decisions/flow links). Config
  // stays a standalone gear icon (design treats settings as a chrome control,
  // not a nav tab — see the header switches cluster below).
  //
  // Built inside the component (not module scope) so it re-renders with the
  // active language — strings pulled from `t`, not a module-level `strings`.
  const NAV: Array<[ViewName, string, IconName]> = [
    ['overview', t.nav.overview, 'layout-dashboard'],
    ['browse', t.nav.browse, 'search'],
  ];
  // Which nav tab lights up for a given route. 概要 owns overview + the legacy
  // #/home landing; ブラウザ owns every browse-family lens/detail route so a
  // deep link (#/spec/<id>, #/decision/<ulid>, #/flow/<action>, …) still shows
  // a highlighted tab instead of none.
  const BROWSE_FAMILY: ViewName[] = ['browse', 'tags', 'spec', 'vocab', 'flow', 'decisions', 'decision'];
  const isActive = (key: ViewName): boolean => {
    if (key === 'overview') return view === 'overview' || view === 'home';
    if (key === 'browse') return BROWSE_FAMILY.includes(view);
    return view === key;
  };

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

  const showFilterToggle = isNarrow && railActive;
  const badgeCount = comments.length;

  return (
    <header class="topbar" ref={headerRef}>
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
        {NAV.map(([key, label, icon]) => (
          <button key={key} type="button" class={'topbar-nav-btn' + (isActive(key) ? ' active' : '')} onClick={() => onSelectView(key)}>
            <Icon name={icon} size={16} />
            <span>{label}</span>
          </button>
        ))}
      </nav>

      <div class={'header-switches' + (showFilterToggle ? ' has-filter-toggle' : '')}>
        {/* E1: narrow 時のみ、絞り込み（ドロワー開閉）をこのツールバー行の左端に
            置く。has-filter-toggle で header-switches を全幅化し、絞り込みの
            margin-right:auto がスペーサになって switches 群を右端へ寄せる。 */}
        {showFilterToggle && (
          <button type="button" class="topbar-filter-toggle" aria-label={t.header.filterToggle} onClick={toggleDrawer}>
            <Icon name="sliders-horizontal" size={15} />
            {t.header.filterToggle}
          </button>
        )}
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
        {/* #27 P2′-rework／AI配送 (change-cockpit-design-v3.md §8.6): badge =
            comment count, scoped to the active task — human comments
            (localStorage) plus AI reviews merged in by useComments (§8.4).
            A comment on a record that has a pending change is a "proposal"
            (rendered with an inline diff card in the drawer — see
            CommentPanel), so it's already counted here without a separate
            tally.
            req.comfortable-viewer.annotations amend: the icon itself is now
            always present (not gated on badgeCount > 0) — it's the only entry
            point that opens CommentPanel without a specific record's
            CommentButton, and with count=0 it opens straight to the panel's
            own "click + on a card to add one" empty state instead of being
            unreachable. Only the numeric badge stays conditional. */}
        <button
          type="button"
          class={'topbar-icon-btn comment-header-btn' + (panelOpen ? ' active' : '')}
          aria-label={t.header.commentList}
          onClick={openPanel}
        >
          <Icon name={badgeCount > 0 ? 'message-filled' : 'message-plus'} size={18} />
          {badgeCount > 0 && <span class="comment-header-badge">{badgeCount}</span>}
        </button>
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
