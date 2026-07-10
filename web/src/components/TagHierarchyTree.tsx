import type { FacetTreeNode } from '../types';
import { strings } from '../strings';

interface Props {
  nodes: FacetTreeNode[];
  depth?: number;
  counts: Record<string, number | undefined>;
  collapsed: Set<string>;
  onToggle: (id: string) => void;
  onBrowse: (id: string) => void;
  onSpec: (id: string) => void;
  onTraceability: (id: string, kind: string) => void;
  traceabilityKinds: string[];
}

export function TagHierarchyTree({
  nodes,
  depth = 0,
  counts,
  collapsed,
  onToggle,
  onBrowse,
  onSpec,
  onTraceability,
  traceabilityKinds,
}: Props) {
  if (nodes.length === 0) return null;
  return (
    <ul class="tag-hier-list">
      {nodes.map((node) => {
        const hasChildren = !!node.children && node.children.length > 0;
        const isCollapsed = collapsed.has(node.tag.id);
        const count = counts[node.tag.id];
        return (
          <li key={node.tag.id}>
            <div class="tag-hier-row" style={{ paddingLeft: `${depth * 1.25}em` }}>
              {hasChildren ? (
                <button
                  type="button"
                  class="tag-hier-toggle"
                  aria-label={isCollapsed ? strings.tags.expandAll : strings.tags.collapseAll}
                  onClick={() => onToggle(node.tag.id)}
                >
                  {isCollapsed ? '▶' : '▼'}
                </button>
              ) : (
                <span class="tag-hier-toggle-spacer" />
              )}
              <span class="tag-hier-name">{node.tag.name || node.tag.id}</span>
              <span class="tag-hier-id dim">{node.tag.id}</span>
              <span class="tag-hier-count dim">{count === undefined ? '…' : strings.tags.txCount(count)}</span>
              <span class="tag-hier-actions">
                <button type="button" onClick={() => onBrowse(node.tag.id)}>
                  {strings.tags.browse}
                </button>
                <button type="button" onClick={() => onSpec(node.tag.id)}>
                  {strings.tags.specLink}
                </button>
                {traceabilityKinds.includes(node.tag.kind || '') && (
                  <button type="button" onClick={() => onTraceability(node.tag.id, node.tag.kind || '')}>
                    {strings.tags.traceability}
                  </button>
                )}
              </span>
            </div>
            {hasChildren && !isCollapsed && (
              <TagHierarchyTree
                nodes={node.children!}
                depth={depth + 1}
                counts={counts}
                collapsed={collapsed}
                onToggle={onToggle}
                onBrowse={onBrowse}
                onSpec={onSpec}
                onTraceability={onTraceability}
                traceabilityKinds={traceabilityKinds}
              />
            )}
          </li>
        );
      })}
    </ul>
  );
}
