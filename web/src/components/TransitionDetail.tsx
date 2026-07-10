import { useEffect, useState } from 'preact/hooks';
import { api } from '../api';
import type { TransitionDetail as TxDetail } from '../types';

interface Props {
  txId?: string;
}

export function TransitionDetailPanel({ txId }: Props) {
  const [detail, setDetail] = useState<TxDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

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
      <h2>{detail.id}</h2>
      <p>
        <strong>action:</strong> {detail.action}
        {detail.actionLabel && ` (${detail.actionLabel})`}
      </p>
      <p>
        <strong>given:</strong>
      </p>
      <ul>
        {detail.given.map((g, i) => (
          <li key={g}>
            {g}
            {detail.givenLabels?.[i] && ` (${detail.givenLabels[i]})`}
          </li>
        ))}
      </ul>
      <p>
        <strong>then:</strong>
      </p>
      <ol>
        {detail.then.map((e, i) => (
          <li key={e}>
            {e}
            {detail.thenLabels?.[i] && ` (${detail.thenLabels[i]})`}
          </li>
        ))}
      </ol>
      {detail.effectiveTags && detail.effectiveTags.length > 0 && (
        <>
          <p>
            <strong>effective tags:</strong>
          </p>
          <p>{detail.effectiveTags.join(', ')}</p>
        </>
      )}
      {detail.tests && detail.tests.length > 0 && (
        <>
          <p>
            <strong>tests:</strong>
          </p>
          <p>{detail.tests.join(', ')}</p>
        </>
      )}
      {detail.rules && detail.rules.length > 0 && (
        <>
          <p>
            <strong>rules:</strong>
          </p>
          <ul>
            {detail.rules.map((r) => (
              <li key={r.id}>
                {r.why}
                {r.ref && ` (${r.ref})`}
              </li>
            ))}
          </ul>
        </>
      )}
    </aside>
  );
}
