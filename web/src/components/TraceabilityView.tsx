import { useEffect, useState } from 'preact/hooks';
import { api } from '../api';
import { useLookups } from '../lookups';
import type { TraceabilityResponse } from '../types';

interface Props {
  onSelectTx: (id: string) => void;
  initialKind?: string;
}

export function TraceabilityView({ onSelectTx, initialKind }: Props) {
  const [data, setData] = useState<TraceabilityResponse | null>(null);
  const [activeKind, setActiveKind] = useState('');
  const [error, setError] = useState<string | null>(null);
  const { transitionLabel } = useLookups();

  useEffect(() => {
    api
      .getTraceability()
      .then((res) => {
        setData(res);
        if (initialKind && res.kinds.includes(initialKind)) setActiveKind(initialKind);
        else if (res.kinds.length > 0) setActiveKind(res.kinds[0]);
      })
      .catch((err) => setError(String(err)));
  }, [initialKind]);

  if (error) return <main class="traceability error">{error}</main>;
  if (!data) return <main class="traceability dim">loading…</main>;

  const entries = data.entries.filter((e) => e.tag.kind === activeKind);
  const gapCount = entries.filter((e) => e.gap).length;

  return (
    <main class="traceability">
      <h2>Traceability</h2>
      {data.kinds.length > 1 && (
        <div class="facet-tabs">
          {data.kinds.map((kind) => (
            <button
              key={kind}
              type="button"
              class={'facet-tab' + (kind === activeKind ? ' active' : '')}
              onClick={() => setActiveKind(kind)}
            >
              {kind}
            </button>
          ))}
        </div>
      )}
      <p class="dim">
        {entries.length} 件中 {gapCount} 件が未充足（gap）
      </p>
      {entries.length === 0 && <p class="dim">該当する要件タグはありません</p>}
      <ul class="traceability-list">
        {entries.map((e) => (
          <li key={e.tag.id} class={'traceability-entry' + (e.gap ? ' gap' : '')}>
            <div class="traceability-tag">
              <span class="tag-name">{e.tag.name || e.tag.id}</span>
              <span class="tag-id dim">{e.tag.id}</span>
              {e.gap ? (
                <span class="gap-badge">gap（0 充足）</span>
              ) : (
                <span class="satisfied-count dim">{e.satisfiedBy.length} 件充足</span>
              )}
            </div>
            {!e.gap && (
              <ul class="satisfied-tx-list">
                {e.satisfiedBy.map((txId) => {
                  const label = transitionLabel(txId);
                  return (
                    <li key={txId}>
                      <button type="button" class="tx-row" title={txId} onClick={() => onSelectTx(txId)}>
                        {label.primary}
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
          </li>
        ))}
      </ul>
    </main>
  );
}
