import type { FacetTreeNode, Tag } from '../../types';
import type { IndexItem } from './BrowseRail';

// ── 見出し（rail 索引）の折りたたみ状態 ─────────────────────────────
// Collapse state for the 見出し index, persisted per facet so a reload
// restores which subtrees are folded (依頼1). Stores the set of *collapsed*
// keys, so the default (empty) is "all expanded". Pure localStorage, no
// bearing on the .pmem model — same private-mode-tolerant pattern as
// settings.ts. Shared by the tag facet (BrowseView) and the tag-folder
// index used by the spec/vocab facets (依頼C-1) so all three restore folds
// the same way, each under its own facet key.
export const COLLAPSE_KEY_PREFIX = 'pmem-browse-collapse-';

export function loadCollapsed(facet: string): Set<string> {
  try {
    const raw = localStorage.getItem(COLLAPSE_KEY_PREFIX + facet);
    if (!raw) return new Set();
    const arr: unknown = JSON.parse(raw);
    return Array.isArray(arr) ? new Set(arr.filter((x): x is string => typeof x === 'string')) : new Set();
  } catch {
    return new Set();
  }
}

export function saveCollapsed(facet: string, ids: Set<string>): void {
  try {
    localStorage.setItem(COLLAPSE_KEY_PREFIX + facet, JSON.stringify([...ids]));
  } catch {
    // Private-mode/quota failures still fold this session — just don't persist.
  }
}

// ── タグ階層フォルダ索引（依頼C-1） ─────────────────────────────────
// One vocab/spec item shown in the rail index. `tags` is the item's *own*
// tag ids (Transition.tags / VocabEntry.tags — the confirmed basis, §C-1):
// the item is filed under a folder for each of its own tags, so a multi-tag
// item appears in every matching folder (重複表示・確定仕様). Items whose own
// tags don't resolve to any folder in the forest land in the 未分類 folder,
// so nothing that was visible in the old flat list disappears (退行回避).
export interface FolderLeafInput {
  id: string;
  label: string;
  color: string;
  tags: string[];
}

interface BuildFolderIndexOpts {
  /** The unified tag forest (§3.8) — the folder skeleton. */
  roots: FacetTreeNode[];
  /** Visible items to file into folders, in the order they should list. */
  leaves: FolderLeafInput[];
  /** Label for the trailing 未分類 folder. */
  untaggedLabel: string;
  /** Dot color for a tag folder row (kind color, matching the tag facet). */
  folderColor: (tag: Tag) => string;
  collapsedIds: Set<string>;
  onToggle: (key: string) => void;
  onSelect: (id: string) => void;
}

const UNCATEGORIZED_KEY = '__uncategorized__';

// An internal folder/leaf node built before flattening to IndexItem[]. Keys
// are unique per row (a leaf under two folders gets two distinct keys) so
// React keys never collide and each folder folds independently.
interface FolderNode {
  key: string;
  scrollId: string;
  label: string;
  color: string;
  isFolder: boolean;
  children: FolderNode[];
}

// Builds the rail's tag-hierarchy folder index (依頼C-1): tags become
// collapsible folders (nested by the same forest the tag facet uses) and the
// vocab/spec items file into them by their own tags. Reused by BrowseView's
// spec facet and VocabView so both get the same fold-per-folder behavior as
// the tag facet's outline.
export function buildFolderIndex(opts: BuildFolderIndexOpts): IndexItem[] {
  const { roots, leaves, untaggedLabel, folderColor, collapsedIds, onToggle, onSelect } = opts;

  // Every tag id reachable in the forest — the set of ids that can become a
  // folder. Filing/未分類 detection both key off this same set so a leaf can
  // never fall between "no folder" and "not counted as untagged" (退行回避).
  const forestTagIds = new Set<string>();
  const collectIds = (nodes: FacetTreeNode[]) => {
    for (const n of nodes) {
      forestTagIds.add(n.tag.id);
      if (n.children) collectIds(n.children);
    }
  };
  collectIds(roots);

  const itemsByTag = new Map<string, FolderLeafInput[]>();
  const untagged: FolderLeafInput[] = [];
  for (const leaf of leaves) {
    const own = leaf.tags.filter((tid) => forestTagIds.has(tid));
    if (own.length === 0) {
      untagged.push(leaf);
      continue;
    }
    for (const tid of own) {
      const arr = itemsByTag.get(tid);
      if (arr) arr.push(leaf);
      else itemsByTag.set(tid, [leaf]);
    }
  }

  const leafNode = (folderKey: string, leaf: FolderLeafInput): FolderNode => ({
    key: folderKey + '::' + leaf.id,
    scrollId: leaf.id,
    label: leaf.label,
    color: leaf.color,
    isFolder: false,
    children: [],
  });

  // First-encountered dedup (matches buildTagOrder): the forest is a
  // multi-parent DAG, so a tag can appear under several parents — keep its
  // folder at the first path only. A folder's contents (direct items + child
  // folders) are path-independent, so this loses nothing.
  const seen = new Set<string>();
  const buildFolders = (nodes: FacetTreeNode[]): FolderNode[] => {
    const out: FolderNode[] = [];
    for (const n of nodes) {
      if (seen.has(n.tag.id)) continue;
      seen.add(n.tag.id);
      const key = 'folder:' + n.tag.id;
      const leafRows = (itemsByTag.get(n.tag.id) || []).map((l) => leafNode(key, l));
      const childFolders = buildFolders(n.children || []);
      // Prune empty folders — a tag with no visible items anywhere in its
      // subtree would just be navigational noise in the index.
      if (leafRows.length === 0 && childFolders.length === 0) continue;
      out.push({
        key,
        scrollId: '',
        label: n.tag.name || n.tag.id,
        color: folderColor(n.tag),
        isFolder: true,
        // Direct items first, then narrower sub-tag folders, so a folder's
        // own members sit next to its header rather than below its subtree.
        children: [...leafRows, ...childFolders],
      });
    }
    return out;
  };

  const folders = buildFolders(roots);

  if (untagged.length > 0) {
    const key = 'folder:' + UNCATEGORIZED_KEY;
    folders.push({
      key,
      scrollId: '',
      label: untaggedLabel,
      color: 'var(--lm-text-dim)',
      isFolder: true,
      children: untagged.map((l) => leafNode(key, l)),
    });
  }

  const out: IndexItem[] = [];
  const emit = (nodes: FolderNode[], depth: number) => {
    for (const n of nodes) {
      const hasChildren = n.isFolder && n.children.length > 0;
      const collapsed = collapsedIds.has(n.key);
      out.push({
        id: n.key,
        label: n.label,
        color: n.color,
        indent: depth,
        hasChildren,
        collapsed: hasChildren ? collapsed : undefined,
        onToggle: hasChildren ? () => onToggle(n.key) : undefined,
        // Folder rows fold on click (there's no tag card to jump to on the
        // spec/vocab screens); leaf rows scroll to their card.
        onClick: n.isFolder ? () => onToggle(n.key) : () => onSelect(n.scrollId),
      });
      if (hasChildren && !collapsed) emit(n.children, depth + 1);
    }
  };
  emit(folders, 0);
  return out;
}
