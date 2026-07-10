import type { FacetTreeNode } from '../types';

interface Props {
  nodes: FacetTreeNode[];
  selectedId?: string;
  onSelect: (id: string) => void;
  depth?: number;
}

export function TagTree({ nodes, selectedId, onSelect, depth = 0 }: Props) {
  if (nodes.length === 0) return null;
  return (
    <ul class="tag-tree" style={depth ? { paddingLeft: '1em' } : undefined}>
      {nodes.map((node) => (
        <li key={node.tag.id}>
          <button
            type="button"
            class={'tag-node' + (node.tag.id === selectedId ? ' selected' : '')}
            onClick={() => onSelect(node.tag.id)}
          >
            {node.tag.name || node.tag.id}
          </button>
          {node.children && <TagTree nodes={node.children} selectedId={selectedId} onSelect={onSelect} depth={depth + 1} />}
        </li>
      ))}
    </ul>
  );
}
