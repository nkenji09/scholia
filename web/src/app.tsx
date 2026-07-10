import { useState } from 'preact/hooks';
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

type View = 'browse' | 'vocab' | 'spec' | 'tags' | 'traceability' | 'compare' | 'config';

export function App() {
  const [view, setView] = useState<View>('browse');
  const [selectedTagId, setSelectedTagId] = useState<string | undefined>(undefined);
  const [selectedTxId, setSelectedTxId] = useState<string | undefined>(undefined);
  const [specTagId, setSpecTagId] = useState<string | undefined>(undefined);
  const [traceabilityKind, setTraceabilityKind] = useState<string | undefined>(undefined);

  const openTransition = (txId: string) => {
    setView('browse');
    setSelectedTxId(txId);
  };

  const openTagBrowse = (tagId: string) => {
    setView('browse');
    setSelectedTagId(tagId);
    setSelectedTxId(undefined);
  };

  const openTagSpec = (tagId: string) => {
    setView('spec');
    setSpecTagId(tagId);
  };

  const openTagTraceability = (_tagId: string, kind: string) => {
    setView('traceability');
    setTraceabilityKind(kind);
  };

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
            selectedTagId={selectedTagId}
            onSelectTag={(id) => {
              setSelectedTagId(id);
              setSelectedTxId(undefined);
            }}
          />
          <TransitionList tagId={selectedTagId} selectedTxId={selectedTxId} onSelectTx={setSelectedTxId} />
          <TransitionDetailPanel txId={selectedTxId} />
        </div>
      )}
      {view === 'vocab' && <VocabView onSelectTx={openTransition} />}
      {view === 'spec' && <SpecView selectedTagId={specTagId} onSelectTag={setSpecTagId} onSelectTx={openTransition} />}
      {view === 'tags' && (
        <TagsView onBrowse={openTagBrowse} onSpec={openTagSpec} onTraceability={openTagTraceability} />
      )}
      {view === 'traceability' && (
        <div class="layout layout-two">
          <TraceabilityView onSelectTx={openTransition} initialKind={traceabilityKind} />
          <TransitionDetailPanel txId={selectedTxId} />
        </div>
      )}
      {view === 'compare' && !isStaticMode && <CompareView />}
      {view === 'config' && <ConfigView />}
    </>
  );
}
