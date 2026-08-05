package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

func newTagListCmd() *cobra.Command {
	var kind string
	var tree, asJSON, all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "タグを一覧する（--tree で parentIds の入れ子表示・--json の既定は description を畳む）",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// ⚠️ **この関数にスナップショットは無い。** 何を渡すかの判断
			// （tag_list_fold.go の純関数）は loadTagListTags の中で通り、
			// ここへ返ってくるのは**畳んだ後の値だけ**である。
			// ここに面を足しても、畳む前のタグを掴む変数が無い。
			tags, err := loadTagListTags(kind, all)
			if err != nil {
				return err
			}
			return writeTagList(cmd.OutOrStdout(), tags, kind, tree, asJSON)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "kind で絞り込む")
	cmd.Flags().BoolVar(&tree, "tree", false, "parentIds の入れ子で表示する（多親は複数箇所に出現）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する（--tree と併せても、既定は各タグの description を畳む。全文は --all か show tag）")
	cmd.Flags().BoolVar(&all, "all", false, "畳んでいるものを全部開く（--json の description を全文で出す。テキストの面は元から出していないので変わらない）")
	return cmd
}

// loadTagListTags は store を読み、**畳んだ後のタグ列だけ**を返す。
//
// ⚠️ **この関数の役目は、畳む前の値をスコープから消すことである。**
//
// 前の形は、判断を面の分岐より手前に置きつつ、スナップショットを同じスコープに
// 残していた。**新しい面が畳んだ側ではなくスナップショット側を書けば、判断を
// 素通りできた**——クリーンルームレビューがその変異（`--roots --json` が
// スナップショットを直に出す）を入れ、既存の歯止めは 1 つも落ちなかった。
// 「面ごとに分岐を書かない」と名乗りながら、素通りの経路は残っていた。
//
// ここは CLAUDE.md 3 の一般化である——**あるゲートの内側にいるかを見て回るのを
// やめ、ゲートを通っていない値そのものを描画側から消す。** 呼び出し側は畳んだ
// 値しか受け取れないので、面を足す人が畳む前のタグを掴む変数が存在しない。
//
// 🔴 **塞ぎ切れない残り（正直に名乗る）**: この関数を使わず、面の中で
// `openStore()` と `LoadAll()` を自分で呼び直せば、畳む前のタグは手に入る。
// **それは「うっかり」では書けない**（store を開き直す 5 行が要る）が、
// 書けば通る。bool フラグで表現される面については
// TestTagListEveryDiscoveredFaceFolds が呼び直しの有無に関係なく落とす。
// **bool フラグ以外で表現される面**（位置引数・文字列フラグの値で分岐する面）
// **を、自分で store を開き直して実装した場合だけが、両方の外に出る。**
func loadTagListTags(kind string, all bool) ([]model.Tag, error) {
	s, err := openStore()
	if err != nil {
		return nil, err
	}
	snap, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	if kind != "" && !containsStr(snap.Config.TagKindIDs(), kind) {
		return nil, fmt.Errorf("--kind %q は config.tagKinds に未宣言です", kind)
	}
	// `--json` の面だけでなく**全部の面**に同じ判断を通す。テキストの面は
	// description を 1 文字も出していないので出力は 1 バイトも変わらず
	// （golden が押さえている）、代わりに「`--json` のときだけ畳む」という
	// 条件そのものが消える——`--json` を経ない新しい面が畳む前の値を
	// 受け取る経路が無くなる。
	return foldTagDescriptions(snap.Tags, all), nil
}

// writeTagList は渡されたタグ列を、指定された面の形で書き出す。
//
// **畳む/畳まないの判断はここには無い。** ここは並べ方と書き出しだけで、
// 受け取るタグ列は既に判断を通っている。
func writeTagList(w io.Writer, tags []model.Tag, kind string, tree, asJSON bool) error {
	if tree {
		forest := buildTagForest(tags, kind)
		if asJSON {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			return enc.Encode(forest)
		}
		if len(forest) == 0 {
			fmt.Fprintln(w, "(該当するタグはありません)")
			return nil
		}
		for _, root := range forest {
			printTagNode(w, root, 0)
		}
		return nil
	}

	flat := filterTagsByKind(tags, kind)
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(flat)
	}
	if len(flat) == 0 {
		fmt.Fprintln(w, "(該当するタグはありません)")
		return nil
	}
	for _, t := range flat {
		fmt.Fprintf(w, "%s\t%s\n", t.ID, t.Name)
	}
	return nil
}

func filterTagsByKind(tags []model.Tag, kind string) []model.Tag {
	out := make([]model.Tag, 0, len(tags))
	for _, t := range tags {
		if kind == "" || t.Kind == kind {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

type tagNode struct {
	Tag      model.Tag `json:"tag"`
	Children []tagNode `json:"children,omitempty"`
}

// buildTagForest nests tags by parentIds into a forest, optionally
// restricted to a single kind (a node only counts a parent if that parent
// is also in the kind-filtered set — same rule index.FacetTree uses for
// tagKind-scoped facet trees, but generalized to "no filter" for the
// unfiltered --tree case, which index.FacetTree doesn't support since it
// always takes a single required kind). A tag with parents outside the set
// becomes a root; multi-parent tags appear once under each in-set parent
// (§6 「多親は複数箇所に出現可」).
func buildTagForest(tags []model.Tag, kind string) []tagNode {
	included := make(map[string]model.Tag)
	for _, t := range tags {
		if kind == "" || t.Kind == kind {
			included[t.ID] = t
		}
	}

	childrenOf := make(map[string][]string)
	var roots []string
	for id, t := range included {
		hasParentInSet := false
		for _, p := range t.ParentIDs {
			if _, ok := included[p]; ok {
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
	var build func(id string) tagNode
	build = func(id string) tagNode {
		node := tagNode{Tag: included[id]}
		if onPath[id] {
			return node // 循環防止（正常な記録では tag-ref lint が既に禁止・§5）
		}
		onPath[id] = true
		for _, c := range childrenOf[id] {
			node.Children = append(node.Children, build(c))
		}
		delete(onPath, id)
		return node
	}

	forest := make([]tagNode, 0, len(roots))
	for _, r := range roots {
		forest = append(forest, build(r))
	}
	return forest
}

func printTagNode(w io.Writer, node tagNode, depth int) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(w, "%s- %s (%s)\n", indent, node.Tag.ID, node.Tag.Name)
	for _, c := range node.Children {
		printTagNode(w, c, depth+1)
	}
}
