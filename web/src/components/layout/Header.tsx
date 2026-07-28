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

  // ナビは「概要 / タグ / 意思決定」の3つ（01KYKS4Y56FAHRVCWKMQJK4RT6）。
  //
  // 直前は「概要 / ブラウザ」の2つで、「ブラウザ」1つが**7つの画面**
  // （遷移の一覧・タグの一覧・タグの詳細・語彙・フロー・意思決定の一覧・意思決定の
  // 単票）で点灯していた。どれを見ていてもタブの見た目が同じなので、いま何を見て
  // いるのかがナビから読めない。利用者の言葉:「同じ decisions 画面が、たまにタグ
  // 一覧や詳細になったりしていて、体験としておかしい」。
  //
  // これは 01KYCC2TDC6PGKPVV6DY90BHR4（2タブ再設計）の**部分的な巻き戻し**である。
  // 利用者はそれを自覚したうえで（「回帰に見えるかもしれないけれど」）指示した。
  // ただし同 decision の理由——「どのデータ型を見るか」を先に選ばせる構造をやめる
  // ——は捨てていない: 3つは**読む目的**の分割（俯瞰する／分類から降りる／規則を
  // 読む）であって、レコード型の一覧ではない。だから語彙・フロー・遷移は
  // 引き続きタブを名乗らず、タグ・遷移のカードから降りる。
  //
  // 失うもの（decision で開示済み）: 遷移（仕様）を横断で一覧する入口が、タブと
  // しては無くなる。タグのカードの「関連仕様」から辿る形は残る。
  //
  // Config stays a standalone gear icon (design treats settings as a chrome
  // control, not a nav tab — see the header switches cluster below).
  //
  // Built inside the component (not module scope) so it re-renders with the
  // active language — strings pulled from `t`, not a module-level `strings`.
  const NAV: Array<[ViewName, string, IconName]> = [
    ['overview', t.nav.overview, 'layout-dashboard'],
    ['tags', t.nav.tags, 'tags'],
    ['decisions', t.nav.decisions, 'gavel'],
  ];
  // どのタブが点灯するか。概要は overview ＋ 旧ランディング #/home。タグは
  // タグ・遷移・語彙・フローの各レンズ（deep link で来ても点灯しないタブが無い
  // 状態にしない）。意思決定は一覧と、転送で残した旧単票の URL。
  const TAGS_FAMILY: ViewName[] = ['tags', 'spec', 'browse', 'vocab', 'flow'];
  const DECISIONS_FAMILY: ViewName[] = ['decisions', 'decision'];
  const isActive = (key: ViewName): boolean => {
    if (key === 'overview') return view === 'overview' || view === 'home';
    if (key === 'tags') return TAGS_FAMILY.includes(view);
    if (key === 'decisions') return DECISIONS_FAMILY.includes(view);
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
