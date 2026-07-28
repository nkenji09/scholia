import { useEffect, useRef } from 'preact/hooks';
import { Header } from './components/layout/Header';
import { OverviewView } from './components/overview/OverviewView';
import { BrowseView } from './components/browse/BrowseView';
import { ConfigView } from './components/config/ConfigView';
import { VocabView } from './components/VocabView';
import { FlowView } from './components/FlowView';
import { FlowIndexView } from './components/FlowIndexView';
import type { FlowFilterState } from './components/FlowIndexView';
import { DecisionsView } from './components/decisions/DecisionsView';
import type { DecisionFilterState } from './components/decisions/DecisionsView';
import { DecisionPermalinkRedirect } from './components/decisions/DecisionPermalinkRedirect';
import { DEFAULT_SCOPE } from './components/decisions/decisionScope';
import { CommentPanel } from './components/comments/CommentPanel';
import { useComments } from './components/comments/useComments';
import type { CommentRecord } from './components/comments/useComments';
import { useDrawer } from './drawer';
import { useHashRoute } from './router';
import type { Route, ViewName } from './router';
import type { SearchStateChange } from './components/browse/BrowseView';
import { restoreResizableWidths } from './components/layout/resizableWidths';

// The views that carry per-view search state (view-state-continuity's
// tag/vocab/spec). 'spec' is the legacy per-tag hash rendering the same
// BrowseView as 'tags'; it's a focus route reached via openTagSpec, never a
// nav tab, so its remembered search is written but never read back — harmless.
// 'decisions'/'flow' joined when their lists gained the shared BrowseRail
// filters (viewer-search-consistency).
const SEARCHABLE_VIEWS = new Set<ViewName>(['tags', 'browse', 'spec', 'vocab', 'decisions', 'flow']);

// Which views render a BrowseRail (so Header shows the narrow-viewport 絞り込み
// toggle). 'flow' only does so on its index — the per-action flow diagram
// (route.actionId present) has no rail, so the toggle must not appear there.
function railActiveFor(view: ViewName, hasActionId: boolean): boolean {
  if (view === 'flow') return !hasActionId;
  // 概要 (overview/home) has its own left rail (the structure tree), so the
  // narrow-viewport 絞り込み/drawer toggle applies there too.
  if (view === 'overview' || view === 'home') return true;
  return view === 'tags' || view === 'browse' || view === 'spec' || view === 'vocab' || view === 'decisions';
}

export function App() {
  const [route, navigate] = useHashRoute();
  const view = route.view;
  const { closePanel } = useComments();
  const { closeDrawer } = useDrawer();

  // 左rail/右コメントパネルの保存済み幅を復元（drawer-resize 依頼C-3）。
  useEffect(() => restoreResizableWidths(), []);

  // Per-view search memory (view-state-continuity, act.user.enter-view →
  // restore-search-from-url). Search itself lives in the URL, but the address
  // bar only ever holds the *current* view's search — so a plain nav-tab hop
  // (tags → vocab → tags) would otherwise land on a search-less URL and drop
  // what tags was filtered to. This map remembers each view's last URL search
  // so setView() can reconstruct it; it's an in-session bridge, not a second
  // source of truth (the URL stays authoritative — reload/Back read from it,
  // and leaving a view repopulates the map from the live route below).
  const searchMemory = useRef<
    Map<
      ViewName,
      Pick<
        Route,
        | 'searchQuery'
        | 'searchKindFacet'
        | 'searchFilters'
        | 'searchSubject'
        | 'decisionTargetKind'
        | 'decisionTag'
        | 'decisionCurrency'
        | 'decisionPeriod'
        | 'decisionOn'
        | 'decisionScope'
        | 'flowTags'
      >
    >
  >(new Map());
  useEffect(() => {
    // #/flow is view==='flow' for both its index (has the rail/filters) and a
    // per-action diagram (route.actionId, no filters). Only the index carries
    // search — remembering the diagram would clobber the index's filters with
    // blanks, so skip it.
    if (SEARCHABLE_VIEWS.has(route.view) && !(route.view === 'flow' && route.actionId)) {
      searchMemory.current.set(route.view, {
        searchQuery: route.searchQuery,
        searchKindFacet: route.searchKindFacet,
        searchFilters: route.searchFilters,
        searchSubject: route.searchSubject,
        // DecisionsView filters (#45 D10b-4) — remembered like search state so a
        // plain nav-tab hop back to 意思決定 restores the applied filters too.
        decisionTargetKind: route.decisionTargetKind,
        decisionTag: route.decisionTag,
        decisionCurrency: route.decisionCurrency,
        decisionPeriod: route.decisionPeriod,
        // 対象と向き（01KYKS4Y56FAHRVCWKMQJK4RT6）も他の条件と同じ扱いで覚える
        // ——タブを往復して戻ったときに、絞り込みだけが黙って外れないように。
        decisionOn: route.decisionOn,
        decisionScope: route.decisionScope,
        // #/flow list filters (viewer-search-consistency) — same tab-hop memory.
        flowTags: route.flowTags,
      });
    }
  }, [route]);

  // Design closes the off-canvas rail on every nav/view switch (its
  // setView() sets drawerOpen:false alongside view). Cross-view jumps
  // (openTransition/openTagSpec) go through navigate() same as setView, so
  // watching route.view covers all of them in one place rather than
  // repeating closeDrawer() at each call site below.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => closeDrawer(), [route.view]);

  // Cross-view links (Vocab/Home → BrowseView, etc.) all funnel through
  // navigate() so each hop lands in browser history and Back/Forward step
  // through them one at a time (v2 調整2). 'browse'/'tags'/'spec' are three
  // distinct hash shapes kept for backward compatibility with
  // pre-BROWSE-unification bookmarks (.concierge/decision.md's hash-compat
  // minor decision) — all three now render the same BrowseView, just with
  // a different initial facet/focus.
  const openTransition = (txId: string) => navigate({ view: 'browse', txId });
  // 概要タブの現在地（deep-linking の適用面拡張・01KYGYYMZSS…）。他の focus 系と同じく
  // navigate を通るので、選び直すたびに履歴が1件積まれ、バック1回で直前の現在地へ戻る
  // （粒度の作り分けはしない＝条項(3)）。旧ランディング #/home から選んだ場合も現在地を
  // 持てる #/overview へ寄せる（#/home 自体は入口として引き続き解決する）。
  const openOverviewAt = (cId: string, pId?: string) => navigate({ view: 'overview', componentId: cId, partId: pId });
  const openTagSpec = (tagId: string) => navigate({ view: 'spec', tagId });
  const openVocabEntry = (vocabId: string) => navigate({ view: 'vocab', vocabId });
  // 意思決定の面は一覧1つだけになった（単票は廃止・旧 URL は転送）。ナビタブから
  // 開く経路は setView('decisions') が担い、覚えていた絞り込みもそこで復元される。
  // 1件を開く＝**その1件に絞り込んだ一覧**を開く（01KYKS4Y56FAHRVCWKMQJK4RT6）。
  // 専用の画面へは行かない。他の条件は持ち込まない——名指しの1件に別の絞り込みを
  // 掛けると、置き換え済みの相手を指す導線が既定の効力フィルタで消える。
  const openDecision = (decisionId: string) => navigate({ view: 'decisions', decisionOn: `decision:${decisionId}` });
  // A plain nav-tab hop restores that view's remembered search (see
  // searchMemory above) so the URL round-trips its filters; focus jumps
  // (openTagSpec/openTransition/openVocabEntry) deliberately DON'T, since those
  // carry their own record focus and a stale search would fight it.
  const setView = (next: ViewName) => navigate({ view: next, ...searchMemory.current.get(next) });
  // BrowseView/VocabView's query/kindFacet/filters/subject, mirrored into the
  // current route's hash (url-state-sync): merges onto whatever
  // view/tagId/txId/vocabId are already in `route` rather than replacing them,
  // so search state composes with the existing focus-on-a-card routes instead
  // of clobbering them.
  const onSearchChange = (s: SearchStateChange) =>
    navigate({ ...route, searchQuery: s.query, searchKindFacet: s.kindFacet, searchFilters: s.filtersEncoded, searchSubject: s.subject });
  const browseSearchProps = {
    searchQuery: route.searchQuery || '',
    searchKindFacet: route.searchKindFacet || 'all',
    // Passed through as-is (not `|| ''`) — undefined vs '' is meaningful
    // here, see BrowseView's deriveFilters.
    searchFiltersEncoded: route.searchFilters,
    onSearchChange,
  };
  // recordId for a 'page' comment is the page it was left on (BrowseView's
  // `facet` prop value, or 'vocab') — see CommentButton call sites in
  // BrowseView.tsx/VocabView.tsx.
  const gotoComment = (c: CommentRecord) => {
    if (c.recordType === 'tag') openTagSpec(c.recordId);
    else if (c.recordType === 'transition') openTransition(c.recordId);
    else if (c.recordType === 'vocab') openVocabEntry(c.recordId);
    else if (c.recordType === 'page') setView(c.recordId === 'specs' ? 'browse' : (c.recordId as ViewName));
    closePanel();
  };

  return (
    <>
      <Header view={view} onSelectView={setView} railActive={railActiveFor(view, !!route.actionId)} />
      {(view === 'overview' || view === 'home') && (
        <OverviewView
          componentId={route.componentId}
          partId={route.partId}
          onSelectComponent={openOverviewAt}
          onOpenTag={openTagSpec}
          onOpenTx={openTransition}
        />
      )}
      {view === 'browse' && (
        <BrowseView
          scrollKey="browse"
          facet="specs"
          initialFocusTagId={route.tagId}
          initialFocusTxId={route.txId}
          onGoToSpec={openTransition}
          onGoToVocab={openVocabEntry}
          onGoToTag={openTagSpec}
          {...browseSearchProps}
        />
      )}
      {view === 'vocab' && (
        <VocabView
          scrollKey="vocab"
          onSelectTx={openTransition}
          initialFocusId={route.vocabId}
          searchQuery={route.searchQuery || ''}
          searchCategoryFacet={route.searchKindFacet || 'all'}
          searchFiltersEncoded={route.searchFilters}
          searchSubject={route.searchSubject || ''}
          onSearchChange={onSearchChange}
        />
      )}
      {view === 'spec' && (
        <BrowseView scrollKey="spec" facet="tags" initialFocusTagId={route.tagId} onGoToSpec={openTransition} onGoToVocab={openVocabEntry} onGoToTag={openTagSpec} {...browseSearchProps} />
      )}
      {view === 'tags' && <BrowseView scrollKey="tags" facet="tags" onGoToSpec={openTransition} onGoToVocab={openVocabEntry} onGoToTag={openTagSpec} {...browseSearchProps} />}
      {/* nav「フロー」タブ（tx.viewer.flow-nav-tab）は #/flow の index を出し、
          action を選ぶと #/flow/<action> の既存 FlowView へ（表示内容は不変）。
          index の絞り込み状態（q/k/ft）は URL に載せて復元（viewer-search-
          consistency・flow-browse／deep-linking amend）。 */}
      {view === 'flow' &&
        (route.actionId ? (
          <FlowView actionId={route.actionId} />
        ) : (
          <FlowIndexView
            onSelectAction={(id) => navigate({ view: 'flow', actionId: id })}
            searchQuery={route.searchQuery || ''}
            kindFacet={route.searchKindFacet || 'all'}
            flowTags={route.flowTags || ''}
            onFiltersChange={(f: FlowFilterState) =>
              navigate({
                view: 'flow',
                searchQuery: f.query || undefined,
                searchKindFacet: f.kindFacet === 'all' ? undefined : f.kindFacet,
                flowTags: f.tags.length ? f.tags.join(',') : undefined,
              })
            }
          />
        ))}
      {view === 'decisions' && (
        <DecisionsView
          searchQuery={route.searchQuery || ''}
          // DecisionsView filter state lives in the URL (#45 D10b-4) so
          // reload/Back restore the same 絞り込み. The view resolves defaults
          // ('all'/'') from undefined; onFiltersChange merges every field into
          // the hash at once (a single navigate keeps them composed).
          targetKind={(route.decisionTargetKind || 'all') as DecisionFilterState['targetKind']}
          // Tag filter widened to a comma-joined id list (viewer-search-
          // consistency): '' = no tag filter (not the 'all' sentinel the other
          // axes use). The dt URL key is unchanged; only its value shape grew.
          tagFilter={route.decisionTag || ''}
          // 効いていないものは既定で本文に混ぜない（01KYHW54B8ZXH0NEPH2J7N1X39
          // 条項4）。一覧には畳む器が無いので、既定の絞り込みが「効いているもの
          // だけ」であることがその役目を果たす。'all'/'superseded' は利用者が
          // 明示的に選んだときだけ——なので URL では **'current' を省略側**に置く
          // （他の軸の 'all' 省略とは既定が違う点に注意）。
          currency={(route.decisionCurrency || 'current') as DecisionFilterState['currency']}
          period={(route.decisionPeriod || 'all') as DecisionFilterState['period']}
          // 対象と向き（01KYKS4Y56FAHRVCWKMQJK4RT6）。既定の向き（subtree）は
          // URL に書かない——他の軸の既定省略と同じ扱い。
          on={route.decisionOn || ''}
          scope={route.decisionScope || ''}
          onFiltersChange={(f) =>
            navigate({
              view: 'decisions',
              searchQuery: f.query || undefined,
              decisionTargetKind: f.targetKind === 'all' ? undefined : f.targetKind,
              decisionTag: f.tagFilter || undefined,
              decisionCurrency: f.currency === 'current' ? undefined : f.currency,
              decisionPeriod: f.period === 'all' ? undefined : f.period,
              decisionOn: f.on || undefined,
              decisionScope: f.scope && f.scope !== DEFAULT_SCOPE ? f.scope : undefined,
            })
          }
          onOpenDecision={openDecision}
        />
      )}
      {/* 旧単票の URL は転送で生かす（01KYKS4Y56FAHRVCWKMQJK4RT6）。過去の記録や
          他人に渡したリンクを黙って殺さないための保険で、着く先は同じ中身
          （1件に絞り込んだ一覧＝開いた状態で着地する）。 */}
      {view === 'decision' && <DecisionPermalinkRedirect decisionId={route.decisionId} />}
      {view === 'config' && <ConfigView />}
      <CommentPanel onGoto={gotoComment} />
    </>
  );
}
