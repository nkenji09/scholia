import { useEffect, useState } from 'preact/hooks';
import { api } from '../api';
import type { Transition } from '../types';

interface Props {
  tagId?: string;
  selectedTxId?: string;
  onSelectTx: (id: string) => void;
}

export function TransitionList({ tagId, selectedTxId, onSelectTx }: Props) {
  const [transitions, setTransitions] = useState<Transition[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getTransitions({ tag: tagId })
      .then((res) => setTransitions(res.transitions || []))
      .catch((err) => setError(String(err)));
  }, [tagId]);

  if (error) return <main class="transition-list error">{error}</main>;

  return (
    <main class="transition-list">
      <h2>遷移 ({transitions.length})</h2>
      {transitions.length === 0 && <p class="dim">該当する遷移はありません</p>}
      <ul>
        {transitions.map((t) => (
          <li key={t.id}>
            <button
              type="button"
              class={'tx-row' + (t.id === selectedTxId ? ' selected' : '')}
              onClick={() => onSelectTx(t.id)}
            >
              {t.id}
            </button>
          </li>
        ))}
      </ul>
    </main>
  );
}
