import { useState } from 'preact/hooks';
import { api, ApiError } from '../api';
import type { DecisionDiff, DiffResult, TagDiff, TransitionDiff, VocabDiff } from '../types';

function isVocabTagEmpty(d: VocabDiff | TagDiff): boolean {
  return !d.added?.length && !d.removed?.length && !d.changed?.length;
}

function isTransitionsEmpty(d: TransitionDiff): boolean {
  return !d.added?.length && !d.removed?.length && !d.changed?.length;
}

function isDecisionsEmpty(d: DecisionDiff): boolean {
  return !d.added?.length && !d.removed?.length && !d.changed?.length;
}

export function CompareView() {
  const [ref, setRef] = useState('main');
  const [result, setResult] = useState<DiffResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const runDiff = (e: Event) => {
    e.preventDefault();
    setError(null);
    setLoading(true);
    api
      .getDiff(ref.trim() || undefined)
      .then((res) => setResult(res))
      .catch((err) => {
        setResult(null);
        setError(err instanceof ApiError ? err.message : String(err));
      })
      .finally(() => setLoading(false));
  };

  return (
    <main class="compare">
      <h2>Compare</h2>
      <form class="compare-form" onSubmit={runDiff}>
        <label>
          ref（git ref / branch / commit。既定 main）
          <input value={ref} onInput={(e) => setRef((e.target as HTMLInputElement).value)} placeholder="main" />
        </label>
        <button type="submit" disabled={loading}>
          {loading ? '比較中…' : '比較'}
        </button>
      </form>
      {error && <p class="error">{error}</p>}
      {result && (
        <div class="diff-report">
          <p class="dim">
            作業ツリー vs <code>{result.ref}</code>
          </p>
          <VocabSection title="語彙" diff={result.vocab} labelOf={(v) => `${v.id} (${v.label})`} />
          <TagSection title="タグ" diff={result.tags} />
          <TransitionSection diff={result.transitions} />
          <DecisionSection diff={result.decisions} />
        </div>
      )}
    </main>
  );
}

function VocabSection({ title, diff, labelOf }: { title: string; diff: VocabDiff; labelOf: (v: { id: string; label: string }) => string }) {
  if (isVocabTagEmpty(diff)) {
    return (
      <section class="card diff-section">
        <h3>{title}</h3>
        <p class="dim diff-empty">変更なし</p>
      </section>
    );
  }
  return (
    <section class="card diff-section">
      <h3>{title}</h3>
      <ul class="diff-list">
        {diff.added?.map((v) => (
          <li key={'a' + v.id} class="diff-added">
            + {labelOf(v)}
          </li>
        ))}
        {diff.removed?.map((v) => (
          <li key={'r' + v.id} class="diff-removed">
            − {labelOf(v)}
          </li>
        ))}
        {diff.changed?.map((c) => (
          <li key={'c' + c.id} class="diff-changed">
            ~ {c.id}
          </li>
        ))}
      </ul>
    </section>
  );
}

function TagSection({ title, diff }: { title: string; diff: TagDiff }) {
  if (isVocabTagEmpty(diff)) {
    return (
      <section class="card diff-section">
        <h3>{title}</h3>
        <p class="dim diff-empty">変更なし</p>
      </section>
    );
  }
  return (
    <section class="card diff-section">
      <h3>{title}</h3>
      <ul class="diff-list">
        {diff.added?.map((t) => (
          <li key={'a' + t.id} class="diff-added">
            + {t.id} ({t.name})
          </li>
        ))}
        {diff.removed?.map((t) => (
          <li key={'r' + t.id} class="diff-removed">
            − {t.id} ({t.name})
          </li>
        ))}
        {diff.changed?.map((c) => (
          <li key={'c' + c.id} class="diff-changed">
            ~ {c.id}
          </li>
        ))}
      </ul>
    </section>
  );
}

function TransitionSection({ diff }: { diff: TransitionDiff }) {
  if (isTransitionsEmpty(diff)) {
    return (
      <section class="card diff-section">
        <h3>遷移</h3>
        <p class="dim diff-empty">変更なし</p>
      </section>
    );
  }
  return (
    <section class="card diff-section">
      <h3>遷移</h3>
      <ul class="diff-list">
        {diff.added?.map((t) => (
          <li key={'a' + t.id} class="diff-added">
            + {t.id}
          </li>
        ))}
        {diff.removed?.map((t) => (
          <li key={'r' + t.id} class="diff-removed">
            − {t.id}
          </li>
        ))}
        {diff.changed?.map((c) => (
          <li key={'c' + c.id} class="diff-changed">
            <div>
              ~ {c.id}
              {c.thenReordered && <span class="diff-badge diff-badge-reorder">then 順序変更</span>}
            </div>
            <ul class="diff-sublist">
              {c.actionChanged && (
                <li>
                  action: {c.before.action} → {c.after.action}
                </li>
              )}
              {!!(c.givenAdded?.length || c.givenRemoved?.length) && (
                <li>
                  given: {c.givenAdded?.map((g) => `+${g}`).join(' ')} {c.givenRemoved?.map((g) => `-${g}`).join(' ')}
                </li>
              )}
              {c.thenReordered ? (
                <li>then（順序変更）: [{c.before.then.join(', ')}] → [{c.after.then.join(', ')}]</li>
              ) : (
                c.thenChanged && (
                  <li>
                    then: [{c.before.then.join(', ')}] → [{c.after.then.join(', ')}]
                  </li>
                )
              )}
              {!!(c.tagsAdded?.length || c.tagsRemoved?.length) && (
                <li>
                  tags: {c.tagsAdded?.map((g) => `+${g}`).join(' ')} {c.tagsRemoved?.map((g) => `-${g}`).join(' ')}
                </li>
              )}
              {!!(c.testsAdded?.length || c.testsRemoved?.length) && (
                <li>
                  tests: {c.testsAdded?.map((g) => `+${g}`).join(' ')} {c.testsRemoved?.map((g) => `-${g}`).join(' ')}
                </li>
              )}
            </ul>
          </li>
        ))}
      </ul>
    </section>
  );
}

function DecisionSection({ diff }: { diff: DecisionDiff }) {
  if (isDecisionsEmpty(diff)) {
    return (
      <section class="card diff-section">
        <h3>decisions</h3>
        <p class="dim diff-empty">変更なし</p>
      </section>
    );
  }
  const violation = (diff.removed?.length || 0) > 0 || (diff.changed?.length || 0) > 0;
  return (
    <section class="card diff-section">
      <h3>decisions</h3>
      {violation && <p class="error">append-only 違反を検出しました（decisions の削除／改変）</p>}
      <ul class="diff-list">
        {diff.added?.map((d) => (
          <li key={'a' + d.id} class="diff-added">
            + {d.id} ({d.target.type}: {d.target.id})
          </li>
        ))}
        {diff.removed?.map((d) => (
          <li key={'r' + d.id} class="diff-removed diff-violation">
            ! 削除（append-only 違反）: {d.id} ({d.target.type}: {d.target.id})
          </li>
        ))}
        {diff.changed?.map((c) => (
          <li key={'c' + c.id} class="diff-changed diff-violation">
            ! 改変（append-only 違反）: {c.id}
          </li>
        ))}
      </ul>
    </section>
  );
}
