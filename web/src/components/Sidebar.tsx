import { useEffect, useState } from 'preact/hooks';
import { api } from '../api';
import type { FacetsResponse } from '../types';
import { TagTree } from './TagTree';

interface Props {
  selectedTagId?: string;
  onSelectTag: (id: string | undefined) => void;
}

export function Sidebar({ selectedTagId, onSelectTag }: Props) {
  const [facets, setFacets] = useState<FacetsResponse | null>(null);
  const [activeKind, setActiveKind] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getFacets()
      .then((res) => {
        setFacets(res);
        if (res.facetKinds.length > 0) setActiveKind(res.facetKinds[0]);
      })
      .catch((err) => setError(String(err)));
  }, []);

  if (error) return <aside class="sidebar error">{error}</aside>;
  if (!facets) return <aside class="sidebar dim">loading…</aside>;

  return (
    <aside class="sidebar">
      <div class="facet-tabs">
        {facets.facetKinds.map((kind) => (
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
      <button type="button" class="clear-filter dim" onClick={() => onSelectTag(undefined)}>
        (すべて表示)
      </button>
      <TagTree nodes={facets.trees[activeKind] || []} selectedId={selectedTagId} onSelect={onSelectTag} />
    </aside>
  );
}
