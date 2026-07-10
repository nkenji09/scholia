import { useState } from 'preact/hooks';
import { Sidebar } from './components/Sidebar';
import { TransitionList } from './components/TransitionList';
import { TransitionDetailPanel } from './components/TransitionDetail';
import { ConfigView } from './components/ConfigView';

export function App() {
  const [view, setView] = useState<'browse' | 'config'>('browse');
  const [selectedTagId, setSelectedTagId] = useState<string | undefined>(undefined);
  const [selectedTxId, setSelectedTxId] = useState<string | undefined>(undefined);

  return (
    <>
      <header class="topbar">
        <h1>pmem view</h1>
        <nav>
          <button type="button" class={view === 'browse' ? 'active' : ''} onClick={() => setView('browse')}>
            Browse
          </button>
          <button type="button" class={view === 'config' ? 'active' : ''} onClick={() => setView('config')}>
            Config
          </button>
        </nav>
      </header>
      {view === 'browse' ? (
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
      ) : (
        <ConfigView />
      )}
    </>
  );
}
