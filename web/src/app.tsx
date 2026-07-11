import { isStaticMode } from './api';
import { Header } from './components/layout/Header';
import { HomeView } from './components/home/HomeView';
import { Sidebar } from './components/Sidebar';
import { TransitionList } from './components/TransitionList';
import { TransitionDetailPanel } from './components/TransitionDetail';
import { ConfigView } from './components/ConfigView';
import { TraceabilityView } from './components/TraceabilityView';
import { CompareView } from './components/CompareView';
import { VocabView } from './components/VocabView';
import { SpecView } from './components/SpecView';
import { TagsView } from './components/TagsView';
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
      <Header view={view} onSelectView={setView} onSelectTx={openTransition} />
      {view === 'home' && (
        <HomeView
          onGoTags={() => setView('tags')}
          onGoTraceability={() => setView('traceability')}
          onSelectTag={openTagSpec}
          onSelectTx={openTransition}
        />
      )}
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
