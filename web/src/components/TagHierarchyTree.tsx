import { useState } from 'preact/hooks';
import type { FacetTreeNode } from '../types';
import { strings } from '../strings';
import { Markdown } from './Markdown';

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

function TagHierarchyRow({ node, depth, count, isCollapsed, hasChildren, onToggle, onBrowse, onSpec, onTraceability, traceabilityKinds }: {
  node: FacetTreeNode;
  depth: number;
  count: number | undefined;
  isCollapsed: boolean;
  hasChildren: boolean;
  onToggle: (id: string) => void;
  onBrowse: (id: string) => void;
  onSpec: (id: string) => void;
  onTraceability: (id: string, kind: string) => void;
  traceabilityKinds: string[];
}) {
  const [descOpen, setDescOpen] = useState(false);

  return (
    <>
      <div class="tag-hier-row" style={{ paddingLeft: `${depth * 1.25}em` }} title={node.tag.id}>
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
        {node.tag.description && (
          <button type="button" class="tag-hier-desc-toggle" onClick={() => setDescOpen((o) => !o)}>
            {strings.tags.description}
          </button>
        )}
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
      {descOpen && node.tag.description && (
        <div class="tag-hier-desc" style={{ paddingLeft: `${depth * 1.25 + 1.5}em` }}>
          <Markdown text={node.tag.description} />
        </div>
      )}
    </>
  );
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
            <TagHierarchyRow
              node={node}
              depth={depth}
              count={count}
              isCollapsed={isCollapsed}
              hasChildren={hasChildren}
              onToggle={onToggle}
              onBrowse={onBrowse}
              onSpec={onSpec}
              onTraceability={onTraceability}
              traceabilityKinds={traceabilityKinds}
            />
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
