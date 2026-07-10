import { useEffect, useState } from 'preact/hooks';
import { api } from '../api';
import { useLookups } from '../lookups';
import type { SearchResult } from '../types';

interface Props {
  onSelectTx: (id: string) => void;
}

export function SearchBox({ onSelectTx }: Props) {
  const [query, setQuery] = useState('');
  const [result, setResult] = useState<SearchResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { transitionLabel, describeMatch } = useLookups();

  useEffect(() => {
    const q = query.trim();
    if (!q) {
      setResult(null);
      setError(null);
      return;
    }
    let cancelled = false;
    api
      .search(q)
      .then((res) => {
        if (!cancelled) setResult(res);
      })
      .catch((err) => {
        if (!cancelled) setError(String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [query]);

  const select = (id: string) => {
    setQuery('');
    setResult(null);
    onSelectTx(id);
  };

  return (
    <div class="search-box">
      <input
        type="search"
        class="search-input"
        placeholder="検索（実効タグ・語彙・遷移 id・kind）"
        value={query}
        onInput={(e) => setQuery((e.target as HTMLInputElement).value)}
      />
      {query.trim() && (
        <div class="search-results">
          {error && <p class="error">{error}</p>}
          {result && result.transitions.length === 0 && !error && <p class="dim">該当なし</p>}
          {result && result.transitions.length > 0 && (
            <ul>
              {result.transitions.map((t) => {
                const label = transitionLabel(t.id);
                return (
                  <li key={t.id}>
                    <button type="button" class="search-result-row" title={t.id} onClick={() => select(t.id)}>
                      <span class="search-result-primary">
                        {label.primary}
                        {label.secondary && <span class="dim"> {label.secondary}</span>}
                      </span>
                      {result.matchedOn[t.id] && (
                        <span class="dim search-matched-on">{result.matchedOn[t.id].map(describeMatch).join('、')}</span>
                      )}
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
