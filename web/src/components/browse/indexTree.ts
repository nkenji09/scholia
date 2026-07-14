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

// ── 索引の表示モード永続（vocab-tree-mode） ─────────────────────────
// 語彙索引を「モードA=category×kind / モードB=消費 transition 文脈」で切り替える
// （req.comfortable-viewer.vocab-tree-mode）。選択は再訪時に保つよう localStorage
// へ永続する — 既存の折りたたみ永続（loadCollapsed/saveCollapsed・
// eff.state.section-visibility 系）と同じ private-mode 耐性の流儀で、モデル
// （.pmem）には一切触れない純 UI 状態。
//
// 既定は『文脈』（モードB）: vocab は消費 transition の下でこそ意味を持つ、という
// faceted-nav の見方を初回体験の既定に据える（decide: 既定索引モードを『文脈』に）。
// category×kind（モードA=『分類』）は置換ではなく明示切替で残る追加レンズ。
// localStorage に保存値があればそれを優先し、未保存/不正値のときだけ文脈へ落とす。
export type VocabIndexMode = 'category-kind' | 'transition';
const INDEX_MODE_KEY = 'pmem-vocab-index-mode';

export function loadIndexMode(): VocabIndexMode {
  try {
    // 明示的に 'category-kind' を保存した再訪者のみ分類。未保存/不正値は文脈を既定に。
    return localStorage.getItem(INDEX_MODE_KEY) === 'category-kind' ? 'category-kind' : 'transition';
  } catch {
    return 'transition';
  }
}

export function saveIndexMode(mode: VocabIndexMode): void {
  try {
    localStorage.setItem(INDEX_MODE_KEY, mode);
  } catch {
    // Private-mode/quota failures still switch this session — just don't persist.
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

// ── category→kind ツリー索引（vocab-view-p1 依頼・Phase 1） ─────────────
// vocab は必ず category(action/condition/effect)×kind を持つ intrinsic な軸
// なので、タグのフォレストを流用する buildFolderIndex とは別に、固定2階層の
// 静的タクソノミとして組む: category（最上位）→ kind（第2階層）→ vocab
// （leaf）。タグは二次的な横断フィルタに降格（VocabView の filters がそのまま
// 適用）— このツリーの軸には現れない。全 vocab が category×kind を持つので
// buildFolderIndex の「タグ無し vocab が未分類にフラットに落ちる」問題は
// この軸では原理的に起こらない。kind 未設定の vocab だけ、その category
// 直下の「その他」バケットへ（退行回避）。
export interface CategoryKindLeafInput {
  id: string;
  label: string;
  color: string;
  category: string;
  kind?: string;
}

interface BuildCategoryKindIndexOpts {
  leaves: CategoryKindLeafInput[];
  /** 最上位フォルダの並び（category id、表示順）。 */
  categories: string[];
  categoryLabel: (category: string) => string;
  /** kind の表示順（config.kinds[category] の宣言順）を返す。宣言に無い
      kind が実データに現れた場合は呼び出し側の解決順に任せる（ここでは
      末尾にアルファベット順で追加し、取りこぼしを防ぐ）。 */
  kindOrder: (category: string) => string[];
  /** kind 未設定の vocab を落とす末尾バケットのラベル（例: 「その他」）。 */
  otherKindLabel: string;
  /** category/kind 両フォルダの行の色（category id から解決）。 */
  folderColor: (category: string) => string;
  collapsedIds: Set<string>;
  onToggle: (key: string) => void;
  onSelect: (id: string) => void;
}

export function buildCategoryKindIndex(opts: BuildCategoryKindIndexOpts): IndexItem[] {
  const { leaves, categories, categoryLabel, kindOrder, otherKindLabel, folderColor, collapsedIds, onToggle, onSelect } = opts;

  const out: IndexItem[] = [];
  for (const category of categories) {
    const catLeaves = leaves.filter((l) => l.category === category);
    if (catLeaves.length === 0) continue;

    const catKey = 'catkind:' + category;
    const catCollapsed = collapsedIds.has(catKey);
    out.push({
      id: catKey,
      label: categoryLabel(category),
      color: folderColor(category),
      indent: 0,
      hasChildren: true,
      collapsed: catCollapsed,
      onToggle: () => onToggle(catKey),
      onClick: () => onToggle(catKey),
    });
    if (catCollapsed) continue;

    const byKind = new Map<string, CategoryKindLeafInput[]>();
    const otherLeaves: CategoryKindLeafInput[] = [];
    for (const l of catLeaves) {
      if (!l.kind) {
        otherLeaves.push(l);
        continue;
      }
      const arr = byKind.get(l.kind);
      if (arr) arr.push(l);
      else byKind.set(l.kind, [l]);
    }

    // Declared kinds first (config order), then any kind present in the data
    // but absent from config.kinds — keeps a stale/undeclared kind visible
    // instead of silently dropping its vocab (退行回避).
    const declared = kindOrder(category);
    const declaredSet = new Set(declared);
    const extra = Array.from(byKind.keys())
      .filter((k) => !declaredSet.has(k))
      .sort();

    const emitKindFolder = (key: string, label: string, kindLeaves: CategoryKindLeafInput[]) => {
      if (kindLeaves.length === 0) return;
      const kindKey = catKey + '::' + key;
      const kindCollapsed = collapsedIds.has(kindKey);
      out.push({
        id: kindKey,
        label,
        color: folderColor(category),
        indent: 1,
        hasChildren: true,
        collapsed: kindCollapsed,
        onToggle: () => onToggle(kindKey),
        onClick: () => onToggle(kindKey),
      });
      if (kindCollapsed) return;
      for (const l of kindLeaves) {
        out.push({
          id: kindKey + '::' + l.id,
          label: l.label,
          color: l.color,
          indent: 2,
          onClick: () => onSelect(l.id),
        });
      }
    };

    for (const kind of [...declared, ...extra]) emitKindFolder(kind, kind, byKind.get(kind) || []);
    emitKindFolder('__other__', otherKindLabel, otherLeaves);
  }

  return out;
}

// ── 消費 transition 文脈ツリー索引（モードB・vocab-tree-mode） ───────────
// vocab は「消費される transition の下」で意味を持つ、という faceted-nav の
// per-component 導出（subject→実効タグに subject を含む遷移→参照 vocab）を索引の
// 軸に据えるモードB（eff.state.tree-mode-b-structure）。上位のコンポ/タグ階層は
// selector（subject コンボ）に寄せ、ここは選択スコープ内で「Transition →
// その transition が参照する vocab（前提/結果の色バッジ leaf）」の
// 2 階層に畳む。vocab に直接タグを振らず消費 transition を橋渡しに階層位置を得る
// ので faceted-nav decision と非矛盾（category×kind の置換ではなく追加レンズ）。
//
// transition ノード名（decide: transition ノードは action(WHEN) ラベルで表示）:
// transition は {id,action,given,then} で label 欄を持たないので、その action
// vocab の label（WHEN 句＝「…したとき」）をノードの代表名にする。呼び出し側で
// action→label を解決して `label` に渡す（解決できない稀ケースは未指定→id へ
// フォールバック）。action がノード名（きっかけ）になるので leaf からは action を
// 落とし、refs は given(前提)＋then(結果) のみ渡す（ノード=きっかけ・leaf=前提/結果）。
// どの scope 内 transition にも消費されない vocab は末尾の「未使用」バケットへ
// 集約する（eff.state.tree-mode-b-unused-bucket）。折りたたみは buildFolderIndex /
// buildCategoryKindIndex と同じ collapsedIds/onToggle 基盤を流用（キー接頭辞
// tx: で mode A の catkind: と衝突しない）。
export interface TransitionVocabLeaf {
  id: string;
  label: string;
  /** 役割（きっかけ/前提/結果）＝ vocab.category の色。呼び出し側で解決する。 */
  color: string;
}

interface BuildTransitionVocabIndexOpts {
  /** 選択スコープ内の transitions（描画順＝id 昇順で渡す）。refs は leaf 化する
      vocab id 列（役割順＝前提→結果＝given→then）。action は refs に含めず、
      その label を `label`（ノードの代表名＝WHEN 句）に渡す。`label` 未指定/空の
      ときはノード名を id にフォールバックする。 */
  transitions: { id: string; label?: string; refs: string[] }[];
  /** ツリーに出しうる vocab の母集合（＝可視 vocab）。id→leaf。transition の
      refs はこの map で解決し、未解決（dangling / スコープ外 / フィルタ外）は
      leaf 化しない。未使用判定もこの母集合を基準にする。 */
  vocabById: Map<string, TransitionVocabLeaf>;
  /** 末尾「未使用」バケットのラベル。 */
  unusedLabel: string;
  /** transition フォルダ行の色。 */
  transitionColor: string;
  /** 「未使用」バケット行の色。 */
  unusedColor: string;
  collapsedIds: Set<string>;
  onToggle: (key: string) => void;
  onSelect: (id: string) => void;
}

const UNUSED_KEY = 'tx:__unused__';

export function buildTransitionVocabIndex(opts: BuildTransitionVocabIndexOpts): IndexItem[] {
  const { transitions, vocabById, unusedLabel, transitionColor, unusedColor, collapsedIds, onToggle, onSelect } = opts;

  const out: IndexItem[] = [];
  // 消費された vocab id（scope 内 transition のいずれかが参照）。未使用判定の逆。
  const used = new Set<string>();

  for (const tx of transitions) {
    // この transition が参照する可視 vocab を役割順・重複排除で leaf 化。
    const seen = new Set<string>();
    const leaves: TransitionVocabLeaf[] = [];
    for (const ref of tx.refs) {
      if (seen.has(ref)) continue;
      const leaf = vocabById.get(ref);
      if (!leaf) continue; // dangling ref・スコープ外・フィルタ外は出さない
      seen.add(ref);
      used.add(ref);
      leaves.push(leaf);
    }
    // 参照可視 vocab が無い transition は索引に出さない（空フォルダはノイズ）。
    if (leaves.length === 0) continue;

    const key = 'tx:' + tx.id;
    const collapsed = collapsedIds.has(key);
    out.push({
      id: key,
      // ノード名＝action(WHEN) ラベル。解決できなければ transition id へ。
      label: tx.label || tx.id,
      color: transitionColor,
      indent: 0,
      hasChildren: true,
      collapsed,
      onToggle: () => onToggle(key),
      onClick: () => onToggle(key),
    });
    if (collapsed) continue;
    for (const leaf of leaves) {
      out.push({
        id: key + '::' + leaf.id,
        label: leaf.label,
        color: leaf.color,
        indent: 1,
        onClick: () => onSelect(leaf.id),
      });
    }
  }

  // 未使用: どの scope 内 transition にも消費されない可視 vocab を末尾バケットへ。
  const unusedLeaves = [...vocabById.values()].filter((v) => !used.has(v.id));
  if (unusedLeaves.length > 0) {
    const collapsed = collapsedIds.has(UNUSED_KEY);
    out.push({
      id: UNUSED_KEY,
      label: unusedLabel,
      color: unusedColor,
      indent: 0,
      hasChildren: true,
      collapsed,
      onToggle: () => onToggle(UNUSED_KEY),
      onClick: () => onToggle(UNUSED_KEY),
    });
    if (!collapsed) {
      for (const leaf of unusedLeaves) {
        out.push({
          id: UNUSED_KEY + '::' + leaf.id,
          label: leaf.label,
          color: leaf.color,
          indent: 1,
          onClick: () => onSelect(leaf.id),
        });
      }
    }
  }

  return out;
}
