import { useEffect, useMemo, useState } from 'preact/hooks';
import { api } from '../api';
import { strings } from '../strings';
import type { FacetsResponse, FacetTreeNode } from '../types';
import { TagHierarchyTree } from './TagHierarchyTree';

interface Props {
  onBrowse: (tagId: string) => void;
  onSpec: (tagId: string) => void;
  onTraceability: (tagId: string, kind: string) => void;
}

function flattenIds(nodes: FacetTreeNode[], out: string[] = []): string[] {
  for (const n of nodes) {
    out.push(n.tag.id);
    if (n.children) flattenIds(n.children, out);
  }
  return out;
}

function treeDepth(nodes: FacetTreeNode[]): number {
  let max = 0;
  for (const n of nodes) {
    const childDepth = n.children && n.children.length > 0 ? treeDepth(n.children) : 0;
    max = Math.max(max, 1 + childDepth);
  }
  return max;
}

export function TagsView({ onBrowse, onSpec, onTraceability }: Props) {
  const [facets, setFacets] = useState<FacetsResponse | null>(null);
  const [traceabilityKinds, setTraceabilityKinds] = useState<string[]>([]);
  const [activeKind, setActiveKind] = useState('');
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [counts, setCounts] = useState<Record<string, number | undefined>>({});
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([api.getFacets(), api.getConfig()])
      .then(([f, cfg]) => {
        setFacets(f);
        setTraceabilityKinds(cfg.traceabilityKinds);
        if (f.facetKinds.length > 0) setActiveKind(f.facetKinds[0]);
      })
      .catch((err) => setError(String(err)));
  }, []);

  const tree = facets?.trees[activeKind] || [];
  const ids = useMemo(() => flattenIds(tree), [tree]);

  useEffect(() => {
    if (ids.length === 0) return;
    let cancelled = false;
    setCounts({});
    Promise.all(
      ids.map((id) =>
        api
          .getTransitions({ tag: id })
          .then((res) => [id, (res.transitions || []).length] as const)
          .catch(() => [id, undefined] as const),
      ),
    ).then((pairs) => {
      if (cancelled) return;
      const next: Record<string, number | undefined> = {};
      for (const [id, count] of pairs) next[id] = count;
      setCounts(next);
    });
    return () => {
      cancelled = true;
    };
  }, [ids]);

  if (error) return <main class="tags-view error">{error}</main>;
  if (!facets) return <main class="tags-view dim">{strings.tags.loading}</main>;

  const depth = treeDepth(tree);

  return (
    <main class="tags-view">
      <h2>{strings.tags.heading}</h2>
      <p class="dim">{strings.tags.intro}</p>
      {facets.facetKinds.length > 1 && (
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
      )}
      <div class="tags-view-toolbar">
        <span class="dim">{strings.tags.stats(ids.length, depth)}</span>
        <button type="button" onClick={() => setCollapsed(new Set())}>
          {strings.tags.expandAll}
        </button>
        <button type="button" onClick={() => setCollapsed(new Set(ids))}>
          {strings.tags.collapseAll}
        </button>
      </div>
      {tree.length === 0 && <p class="dim">{strings.tags.empty}</p>}
      <TagHierarchyTree
        nodes={tree}
        counts={counts}
        collapsed={collapsed}
        onToggle={(id) =>
          setCollapsed((prev) => {
            const next = new Set(prev);
            if (next.has(id)) next.delete(id);
            else next.add(id);
            return next;
          })
        }
        onBrowse={onBrowse}
        onSpec={onSpec}
        onTraceability={onTraceability}
        traceabilityKinds={traceabilityKinds}
      />
    </main>
  );
}
