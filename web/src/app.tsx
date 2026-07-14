import { useEffect, useRef } from 'preact/hooks';
import { Header } from './components/layout/Header';
import { HomeView } from './components/home/HomeView';
import { BrowseView } from './components/browse/BrowseView';
import { ConfigView } from './components/config/ConfigView';
import { VocabView } from './components/VocabView';
import { CommentPanel } from './components/comments/CommentPanel';
import { useComments } from './components/comments/useComments';
import type { CommentRecord } from './components/comments/useComments';
import { useDrawer } from './drawer';
import { useHashRoute } from './router';
import type { Route, ViewName } from './router';
import type { SearchStateChange } from './components/browse/BrowseView';
import { restoreResizableWidths } from './components/layout/resizableWidths';

// The three views that carry per-view search state (view-state-continuity's
// tag/vocab/spec). 'spec' is the legacy per-tag hash rendering the same
// BrowseView as 'tags'; it's a focus route reached via openTagSpec, never a
// nav tab, so its remembered search is written but never read back — harmless.
const SEARCHABLE_VIEWS = new Set<ViewName>(['tags', 'browse', 'spec', 'vocab']);

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
  const searchMemory = useRef<Map<ViewName, Pick<Route, 'searchQuery' | 'searchKindFacet' | 'searchFilters' | 'searchSubject'>>>(new Map());
  useEffect(() => {
    if (SEARCHABLE_VIEWS.has(route.view)) {
      searchMemory.current.set(route.view, {
        searchQuery: route.searchQuery,
        searchKindFacet: route.searchKindFacet,
        searchFilters: route.searchFilters,
        searchSubject: route.searchSubject,
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
  const openTagSpec = (tagId: string) => navigate({ view: 'spec', tagId });
  const openVocabEntry = (vocabId: string) => navigate({ view: 'vocab', vocabId });
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
      <Header view={view} onSelectView={setView} />
      {view === 'home' && <HomeView onGoTags={() => setView('tags')} onSelectTag={openTagSpec} onSelectTx={openTransition} />}
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
      {view === 'config' && <ConfigView />}
      <CommentPanel onGoto={gotoComment} />
    </>
  );
}
