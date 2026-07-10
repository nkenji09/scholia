import { useEffect, useMemo, useState } from 'preact/hooks';
import { api } from '../api';
import { strings } from '../strings';
import type { Transition, VocabEntry } from '../types';

interface Props {
  onSelectTx: (id: string) => void;
}

const CATEGORIES: Array<{ key: VocabEntry['category']; label: string }> = [
  { key: 'condition', label: 'condition' },
  { key: 'action', label: 'action' },
  { key: 'effect', label: 'effect' },
];

function usedBy(v: VocabEntry, transitions: Transition[]): Transition[] {
  return transitions.filter((t) => t.action === v.id || t.given.includes(v.id) || t.then.includes(v.id));
}

function VocabRow({ v, transitions, onSelectTx }: { v: VocabEntry; transitions: Transition[]; onSelectTx: (id: string) => void }) {
  const [open, setOpen] = useState(false);
  const uses = useMemo(() => usedBy(v, transitions), [v, transitions]);

  return (
    <li class="vocab-row">
      <div class="vocab-row-main">
        <span class="vocab-id">{v.id}</span>
        <span class="vocab-label">{v.label}</span>
        {v.kind && <span class="vocab-kind-badge">{v.kind}</span>}
        {v.owner && (
          <span class="vocab-owner dim">
            {strings.vocab.owner}: {v.owner}
          </span>
        )}
        <button type="button" class="vocab-usage-toggle" onClick={() => setOpen((o) => !o)}>
          {uses.length > 0
            ? `${open ? strings.vocab.hideTransitions : strings.vocab.showTransitions} (${uses.length})`
            : strings.vocab.noUsage}
        </button>
      </div>
      {open && uses.length > 0 && (
        <ul class="vocab-usage-list">
          {uses.map((t) => (
            <li key={t.id}>
              <button type="button" class="tx-row" onClick={() => onSelectTx(t.id)}>
                {t.id}
              </button>
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}

export function VocabView({ onSelectTx }: Props) {
  const [vocab, setVocab] = useState<VocabEntry[] | null>(null);
  const [transitions, setTransitions] = useState<Transition[]>([]);
  const [activeCategory, setActiveCategory] = useState<VocabEntry['category']>('action');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([api.getVocab(), api.getTransitions({})])
      .then(([v, tx]) => {
        setVocab(v);
        setTransitions(tx.transitions || []);
      })
      .catch((err) => setError(String(err)));
  }, []);

  if (error) return <main class="vocab-view error">{error}</main>;
  if (!vocab) return <main class="vocab-view dim">{strings.vocab.loading}</main>;

  const inCategory = vocab.filter((v) => v.category === activeCategory);
  const byKind = new Map<string, VocabEntry[]>();
  for (const v of inCategory) {
    const key = v.kind || '';
    if (!byKind.has(key)) byKind.set(key, []);
    byKind.get(key)!.push(v);
  }
  const kindGroups = Array.from(byKind.entries()).sort(([a], [b]) => a.localeCompare(b));

  return (
    <main class="vocab-view">
      <h2>{strings.vocab.heading}</h2>
      <p class="dim">{strings.vocab.intro}</p>
      <div class="facet-tabs">
        {CATEGORIES.map((c) => (
          <button
            key={c.key}
            type="button"
            class={'facet-tab' + (c.key === activeCategory ? ' active' : '')}
            onClick={() => setActiveCategory(c.key)}
          >
            {c.label} ({vocab.filter((v) => v.category === c.key).length})
          </button>
        ))}
      </div>
      {inCategory.length === 0 && <p class="dim">{strings.vocab.empty}</p>}
      {kindGroups.map(([kind, entries]) => (
        <section key={kind || '(none)'} class="vocab-kind-group">
          <h3>{kind || strings.vocab.kindUnset}</h3>
          <ul class="vocab-list">
            {entries
              .slice()
              .sort((a, b) => a.id.localeCompare(b.id))
              .map((v) => (
                <VocabRow key={v.id} v={v} transitions={transitions} onSelectTx={onSelectTx} />
              ))}
          </ul>
        </section>
      ))}
    </main>
  );
}
