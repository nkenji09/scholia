import { useEffect, useState } from 'preact/hooks';
import { api } from '../api';
import { useLookups } from '../lookups';
import type { TransitionDetail as TxDetail } from '../types';
import { TransitionFlow } from './TransitionFlow';

interface Props {
  txId?: string;
}

export function TransitionDetailPanel({ txId }: Props) {
  const [detail, setDetail] = useState<TxDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { tagName } = useLookups();

  useEffect(() => {
    if (!txId) {
      setDetail(null);
      return;
    }
    api
      .getTransition(txId)
      .then(setDetail)
      .catch((err) => setError(String(err)));
  }, [txId]);

  if (!txId) return <aside class="detail dim">遷移を選択してください</aside>;
  if (error) return <aside class="detail error">{error}</aside>;
  if (!detail) return <aside class="detail dim">loading…</aside>;

  return (
    <aside class="detail">
      <header class="detail-header">
        <h2>{detail.actionLabel || detail.id}</h2>
        <p class="detail-id dim" title="内部 id（リンクのキー。参照時のみ使用）">
          {detail.id}
        </p>
      </header>

      <TransitionFlow actionLabel={detail.actionLabel || detail.action} givenLabels={detail.givenLabels} thenLabels={detail.thenLabels} />

      {detail.effectiveTags && detail.effectiveTags.length > 0 && (
        <section class="detail-section">
          <h3>タグ</h3>
          <div class="tx-row-tags">
            {detail.effectiveTags.map((id) => (
              <span key={id} class="tx-tag-chip">
                {tagName(id)}
              </span>
            ))}
          </div>
        </section>
      )}

      {detail.tests && detail.tests.length > 0 && (
        <section class="detail-section">
          <h3>tests</h3>
          <ul class="detail-tests">
            {detail.tests.map((test) => (
              <li key={test}>{test}</li>
            ))}
          </ul>
        </section>
      )}

      {detail.rules && detail.rules.length > 0 && (
        <section class="detail-section">
          <h3>rules</h3>
          <ul class="detail-rules">
            {detail.rules.map((r) => (
              <li key={r.id}>
                {r.why}
                {r.ref && <span class="dim"> ({r.ref})</span>}
              </li>
            ))}
          </ul>
        </section>
      )}
    </aside>
  );
}
