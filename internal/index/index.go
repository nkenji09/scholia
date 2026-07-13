// Index builds the derived query structures a snapshot supports read-only
// queries over (§3.9): effective tags per transition, tag→transition and
// vocab→transition reverse lookups, and tagKind→tag trees. It is rebuilt
// wholesale from a store.Snapshot on every CLI invocation — it holds no
// state that isn't cheaply recomputed from the files on disk.
package index

import (
	"sort"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

// Index is the in-memory derived index (§3.9). It is read-only: callers get
// query methods, not a place to mutate records (writes go through store).
type Index struct {
	TransitionByID map[string]model.Transition
	TagByID        map[string]model.Tag
	VocabByID      map[string]model.VocabEntry

	// EffectiveTags[txID] は §3.7 の実効タグ（祖先展開済み・ソート済み）。
	EffectiveTags map[string][]string

	tagTransitions   map[string]map[string]bool // tagId -> txId set（実効タグの逆引き）
	tagChildren      map[string][]string        // parentTagId -> 子 tagId（ソート済み・parentIds の逆引き）
	vocabTransitions map[string]map[string]bool // vocabId -> txId set（action/given/then からの参照の逆引き）
	tagVocab         map[string]map[string]bool // tagId -> vocabId set（VocabEntry.Tags 直付与の逆引き・祖先展開なし）
}

// Build は snapshot 全体から派生インデックスを構築する（§3.9）。SQLite は使わない。
func Build(snap *store.Snapshot) *Index {
	ix := &Index{
		TransitionByID:   make(map[string]model.Transition, len(snap.Transitions)),
		TagByID:          make(map[string]model.Tag, len(snap.Tags)),
		VocabByID:        make(map[string]model.VocabEntry, len(snap.Vocab)),
		EffectiveTags:    make(map[string][]string, len(snap.Transitions)),
		tagTransitions:   make(map[string]map[string]bool),
		tagChildren:      make(map[string][]string),
		vocabTransitions: make(map[string]map[string]bool),
		tagVocab:         make(map[string]map[string]bool),
	}

	for _, t := range snap.Tags {
		ix.TagByID[t.ID] = t
	}
	for _, v := range snap.Vocab {
		ix.VocabByID[v.ID] = v
		// vocab→tag 逆引き（VocabEntry.Tags の直付与のみ。実効タグの祖先展開は
		// しない — 「そのタグを直接持つ語彙」を関連語彙として出すため・H3）。
		for _, tagID := range v.Tags {
			if ix.tagVocab[tagID] == nil {
				ix.tagVocab[tagID] = make(map[string]bool)
			}
			ix.tagVocab[tagID][v.ID] = true
		}
	}
	for _, t := range snap.Tags {
		for _, p := range t.ParentIDs {
			ix.tagChildren[p] = append(ix.tagChildren[p], t.ID)
		}
	}
	for p := range ix.tagChildren {
		sort.Strings(ix.tagChildren[p])
	}

	for _, t := range snap.Transitions {
		t := t // ループ変数のコピー（EffectiveTags は *model.Transition を取る）
		ix.TransitionByID[t.ID] = t

		eff := EffectiveTags(snap, &t)
		ix.EffectiveTags[t.ID] = eff
		for _, tagID := range eff {
			if ix.tagTransitions[tagID] == nil {
				ix.tagTransitions[tagID] = make(map[string]bool)
			}
			ix.tagTransitions[tagID][t.ID] = true
		}

		refs := make([]string, 0, 1+len(t.Given)+len(t.Then))
		refs = append(refs, t.Action)
		refs = append(refs, t.Given...)
		refs = append(refs, t.Then...)
		for _, ref := range refs {
			if ix.vocabTransitions[ref] == nil {
				ix.vocabTransitions[ref] = make(map[string]bool)
			}
			ix.vocabTransitions[ref][t.ID] = true
		}
	}

	return ix
}

// AllTransitions は全遷移を id 昇順で返す。
func (ix *Index) AllTransitions() []model.Transition {
	return ix.transitionsFromSet(nil, true)
}

// TransitionsByTag は実効タグに tagID を含む遷移を id 昇順で返す（祖先展開済みなので子孫タグの遷移もヒットする・§3.7）。
func (ix *Index) TransitionsByTag(tagID string) []model.Transition {
	return ix.transitionsFromSet(ix.tagTransitions[tagID], false)
}

// TransitionsByVocab は action/given/then のいずれかで vocabID を参照する遷移を id 昇順で返す。
func (ix *Index) TransitionsByVocab(vocabID string) []model.Transition {
	return ix.transitionsFromSet(ix.vocabTransitions[vocabID], false)
}

// HasEffectiveTag は遷移 txID の実効タグに tagID が含まれるかを返す。
func (ix *Index) HasEffectiveTag(txID, tagID string) bool {
	return ix.tagTransitions[tagID][txID]
}

// VocabBySubject は subject タグ（コンポ）に属す遷移が参照する語彙を id 昇順で
// 返す（vocab-view-p2）。導出は「subject →（実効タグに subject を含む遷移）→
// 遷移の action/given/then が参照する vocab」の順引き。vocab 自体はタグ不要で
// コンポ別に見られるべき（帰属は遷移側の実効タグに持たせる）という decision に
// 基づく。TransitionsByTag が実効タグ（祖先ロールアップ済み）で判定するので、
// 子タグ付きの遷移も親 subject に正しく現れ、共有語彙は該当する全コンポに出る。
// 該当遷移が無ければ空。VocabEntry.Tags 直付与の逆引き（VocabByTag）とは別系統。
func (ix *Index) VocabBySubject(subjectTagID string) []model.VocabEntry {
	seen := make(map[string]bool)
	out := make([]model.VocabEntry, 0)
	for _, t := range ix.TransitionsByTag(subjectTagID) {
		refs := make([]string, 0, 1+len(t.Given)+len(t.Then))
		refs = append(refs, t.Action)
		refs = append(refs, t.Given...)
		refs = append(refs, t.Then...)
		for _, id := range refs {
			if seen[id] {
				continue
			}
			v, ok := ix.VocabByID[id]
			if !ok {
				continue // dangling ref（vocab-ref lint が拾う・§5）は静かに飛ばす
			}
			seen[id] = true
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// VocabByTag は tagID を（直接）持つ語彙を id 昇順で返す（VocabEntry.Tags の
// 逆引き・祖先展開なし・H3 の関連語彙）。
func (ix *Index) VocabByTag(tagID string) []model.VocabEntry {
	set := ix.tagVocab[tagID]
	out := make([]model.VocabEntry, 0, len(set))
	for id := range set {
		out = append(out, ix.VocabByID[id])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (ix *Index) transitionsFromSet(set map[string]bool, all bool) []model.Transition {
	out := make([]model.Transition, 0, len(ix.TransitionByID))
	if all {
		for _, t := range ix.TransitionByID {
			out = append(out, t)
		}
	} else {
		for id := range set {
			out = append(out, ix.TransitionByID[id])
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// TagNode は facet ツリーの 1 ノード（§3.8 の faceted 階層）。
type TagNode struct {
	Tag      model.Tag
	Children []*TagNode
}

// FacetTree は kind == tagKind のタグを parentIds で入れ子にしたフォレストを返す
// （同 kind 内の親を持たないタグがルート）。多親 DAG なので同じタグが複数の親の下に
// 重複して現れうる（多重所属可・§3.8）。循環はパス単位のガードで無限展開を防ぐ
// （tag-ref lint がある正常な記録では発生しない・§5）。
//
// これは kind スコープの木＝traceability（§7・kind ごとの要件行）や
// `pmem list --facet <kind>`（§3.8 の facet 軸別グルーピング）が使う。
// browse ナビの「1本の統一ツリー」は TagForest（kind 非依存）を使う。
func (ix *Index) FacetTree(tagKind string) []*TagNode {
	inKind := make(map[string]bool)
	for id, t := range ix.TagByID {
		if t.Kind == tagKind {
			inKind[id] = true
		}
	}
	return ix.tagForest(inKind)
}

// TagForest は全タグを parentIds で kind 非依存に入れ子にした「1本の統一
// フォレスト」を返す（§3.8 browse の統一ツリー）。kind は木を分割する軸では
// なくノードの属性（バッジ/色/フィルタ）なので、subject の子に requirement が
// ぶら下がる cross-kind 入れ子や、どの facetKind にも属さない kind=null の
// タグも parentIds 通りに現れる（per-kind の FacetTree では脱落していた）。
// 入れ子規則は CLI の `tag list --tree`（無フィルタ経路）と同一。多親 DAG・
// 循環ガードは FacetTree と同じ。
func (ix *Index) TagForest() []*TagNode {
	all := make(map[string]bool, len(ix.TagByID))
	for id := range ix.TagByID {
		all[id] = true
	}
	return ix.tagForest(all)
}

// tagForest は include に含まれるタグだけを parentIds で入れ子にした
// フォレストを返す共有ヘルパ。親が include 内にあれば子として繋ぎ、include
// 内に親を持たないタグをルートにする。多親は各 in-set 親の下に重複して現れ、
// 循環はパス単位のガードで無限展開を防ぐ。FacetTree（kind スコープ）と
// TagForest（統一）が同じ入れ子規則を 1 か所で共有する。
func (ix *Index) tagForest(include map[string]bool) []*TagNode {
	childrenOf := make(map[string][]string)
	var roots []string
	for id := range include {
		hasParentInSet := false
		for _, p := range ix.TagByID[id].ParentIDs {
			if include[p] {
				childrenOf[p] = append(childrenOf[p], id)
				hasParentInSet = true
			}
		}
		if !hasParentInSet {
			roots = append(roots, id)
		}
	}
	sort.Strings(roots)
	for p := range childrenOf {
		sort.Strings(childrenOf[p])
	}

	onPath := make(map[string]bool)
	var build func(id string) *TagNode
	build = func(id string) *TagNode {
		node := &TagNode{Tag: ix.TagByID[id]}
		if onPath[id] {
			return node
		}
		onPath[id] = true
		for _, c := range childrenOf[id] {
			node.Children = append(node.Children, build(c))
		}
		delete(onPath, id)
		return node
	}

	forest := make([]*TagNode, 0, len(roots))
	for _, r := range roots {
		forest = append(forest, build(r))
	}
	return forest
}
