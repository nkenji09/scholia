import { isStaticMode } from './api';
import { Sidebar } from './components/Sidebar';
import { TransitionList } from './components/TransitionList';
import { TransitionDetailPanel } from './components/TransitionDetail';
import { ConfigView } from './components/ConfigView';
import { TraceabilityView } from './components/TraceabilityView';
import { SearchBox } from './components/SearchBox';
import { CompareView } from './components/CompareView';
import { VocabView } from './components/VocabView';
import { SpecView } from './components/SpecView';
import { TagsView } from './components/TagsView';
import { strings } from './strings';
import { useHashRoute } from './router';
import type { ViewName } from './router';

export function App() {
  const [route, navigate] = useHashRoute();
  const view = route.view;

  // Cross-view links (Vocab/Traceability/Tags → Browse or Spec, etc.) all
  // funnel through navigate() so each hop lands in browser history and
  // Back/Forward step through them one at a time (v2 調整2).
  const openTransition = (txId: string) => navigate({ view: 'browse', txId });
  const openTagBrowse = (tagId: string) => navigate({ view: 'browse', tagId });
  const openTagSpec = (tagId: string) => navigate({ view: 'spec', tagId });
  const openTagTraceability = (_tagId: string, kind: string) => navigate({ view: 'traceability', kind });
  const setView = (next: ViewName) => navigate({ view: next });

  return (
    <>
      <header class="topbar">
        <h1>pmem view</h1>
        <SearchBox onSelectTx={openTransition} />
        <nav>
          <button type="button" class={view === 'browse' ? 'active' : ''} onClick={() => setView('browse')}>
            Browse
          </button>
          <button type="button" class={view === 'vocab' ? 'active' : ''} onClick={() => setView('vocab')}>
            {strings.nav.vocab}
          </button>
          <button type="button" class={view === 'spec' ? 'active' : ''} onClick={() => setView('spec')}>
            {strings.nav.spec}
          </button>
          <button type="button" class={view === 'tags' ? 'active' : ''} onClick={() => setView('tags')}>
            {strings.nav.tags}
          </button>
          <button
            type="button"
            class={view === 'traceability' ? 'active' : ''}
            onClick={() => setView('traceability')}
          >
            Traceability
          </button>
          {!isStaticMode && (
            <button type="button" class={view === 'compare' ? 'active' : ''} onClick={() => setView('compare')}>
              Compare
            </button>
          )}
          <button type="button" class={view === 'config' ? 'active' : ''} onClick={() => setView('config')}>
            Config
          </button>
        </nav>
      </header>
      {view === 'browse' && (
        <div class="layout">
          <Sidebar
            selectedTagId={route.tagId}
            onSelectTag={(id) => navigate({ view: 'browse', tagId: id })}
          />
          <TransitionList
            tagId={route.tagId}
            selectedTxId={route.txId}
            onSelectTx={(id) => navigate({ view: 'browse', tagId: route.tagId, txId: id })}
          />
          <TransitionDetailPanel txId={route.txId} />
        </div>
      )}
      {view === 'vocab' && <VocabView onSelectTx={openTransition} />}
      {view === 'spec' && (
        <SpecView
          selectedTagId={route.tagId}
          onSelectTag={(id) => navigate({ view: 'spec', tagId: id })}
          onSelectTx={openTransition}
        />
      )}
      {view === 'tags' && (
        <TagsView onBrowse={openTagBrowse} onSpec={openTagSpec} onTraceability={openTagTraceability} />
      )}
      {view === 'traceability' && (
        <div class="layout layout-two">
          <TraceabilityView onSelectTx={openTransition} initialKind={route.kind} />
          <TransitionDetailPanel txId={route.txId} />
        </div>
      )}
      {view === 'compare' && !isStaticMode && <CompareView />}
      {view === 'config' && <ConfigView />}
    </>
  );
}
