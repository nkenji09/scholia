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
	var tree, asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "タグを一覧する（--tree で parentIds の入れ子表示）",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			if kind != "" && !containsStr(snap.Config.TagKindIDs(), kind) {
				return fmt.Errorf("--kind %q は config.tagKinds に未宣言です", kind)
			}

			if tree {
				forest := buildTagForest(snap.Tags, kind)
				if asJSON {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(forest)
				}
				w := cmd.OutOrStdout()
				if len(forest) == 0 {
					fmt.Fprintln(w, "(該当するタグはありません)")
					return nil
				}
				for _, root := range forest {
					printTagNode(w, root, 0)
				}
				return nil
			}

			flat := filterTagsByKind(snap.Tags, kind)
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(flat)
			}
			w := cmd.OutOrStdout()
			if len(flat) == 0 {
				fmt.Fprintln(w, "(該当するタグはありません)")
				return nil
			}
			for _, t := range flat {
				fmt.Fprintf(w, "%s\t%s\n", t.ID, t.Name)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "kind で絞り込む")
	cmd.Flags().BoolVar(&tree, "tree", false, "parentIds の入れ子で表示する（多親は複数箇所に出現）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
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
