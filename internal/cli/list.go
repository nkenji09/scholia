package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
)

// facetNode は --facet 出力の JSON 表現（index.TagNode に遷移を添えたもの）。
type facetNode struct {
	Tag         model.Tag          `json:"tag"`
	Transitions []model.Transition `json:"transitions,omitempty"`
	Children    []facetNode        `json:"children,omitempty"`
}

type listOutput struct {
	Transitions []model.Transition `json:"transitions,omitempty"`
	Facet       string             `json:"facet,omitempty"`
	Roots       []facetNode        `json:"roots,omitempty"`
	Untagged    []model.Transition `json:"untagged,omitempty"`
}

func newListCmd() *cobra.Command {
	var facet, tagID, kind string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "遷移を faceted / タグ / kind で一覧する（派生ビュー・§3.8）",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			if facet != "" && !containsStr(snap.Config.TagKinds, facet) {
				return fmt.Errorf("--facet %q は config.tagKinds に未宣言です", facet)
			}
			if tagID != "" && !s.TagExists(tagID) {
				return fmt.Errorf("--tag %q が実在しません", tagID)
			}
			if kind != "" && !containsStr(snap.Config.Kinds.Action, kind) {
				return fmt.Errorf("--kind %q は config.kinds.action に未宣言です", kind)
			}

			ix := index.Build(&snap)
			filtered := filterTransitions(ix, ix.AllTransitions(), tagID, kind)

			var out listOutput
			if facet != "" {
				out.Facet = facet
				forest := ix.FacetTree(facet)
				inSet := toTxSet(filtered)
				for _, root := range forest {
					out.Roots = append(out.Roots, buildFacetNode(ix, root, inSet))
				}
				out.Untagged = untaggedTransitions(ix, filtered, facet)
			} else {
				out.Transitions = filtered
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			printList(cmd, out)
			return nil
		},
	}
	cmd.Flags().StringVar(&facet, "facet", "", "その kind のタグ入れ子ツリーで表示する")
	cmd.Flags().StringVar(&tagID, "tag", "", "実効タグに含まれる遷移だけに絞り込む")
	cmd.Flags().StringVar(&kind, "kind", "", "action の kind で絞り込む")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}

// filterTransitions は --tag / --kind を AND で適用する（--kind は action の kind と解釈する。実装判断: result.md 参照）。
func filterTransitions(ix *index.Index, all []model.Transition, tagID, kind string) []model.Transition {
	out := make([]model.Transition, 0, len(all))
	for _, t := range all {
		if tagID != "" && !ix.HasEffectiveTag(t.ID, tagID) {
			continue
		}
		if kind != "" && ix.VocabByID[t.Action].Kind != kind {
			continue
		}
		out = append(out, t)
	}
	return out
}

func toTxSet(ts []model.Transition) map[string]bool {
	set := make(map[string]bool, len(ts))
	for _, t := range ts {
		set[t.ID] = true
	}
	return set
}

func buildFacetNode(ix *index.Index, node *index.TagNode, inSet map[string]bool) facetNode {
	fn := facetNode{Tag: node.Tag}
	for _, t := range ix.TransitionsByTag(node.Tag.ID) {
		if inSet[t.ID] {
			fn.Transitions = append(fn.Transitions, t)
		}
	}
	for _, c := range node.Children {
		fn.Children = append(fn.Children, buildFacetNode(ix, c, inSet))
	}
	return fn
}

// untaggedTransitions はどの kind==facet のタグにもヒットしない遷移（末尾グループ・§3.8）。
func untaggedTransitions(ix *index.Index, filtered []model.Transition, facet string) []model.Transition {
	var out []model.Transition
	for _, t := range filtered {
		hasFacetTag := false
		for _, tagID := range ix.EffectiveTags[t.ID] {
			if ix.TagByID[tagID].Kind == facet {
				hasFacetTag = true
				break
			}
		}
		if !hasFacetTag {
			out = append(out, t)
		}
	}
	return out
}

func printList(cmd *cobra.Command, out listOutput) {
	w := cmd.OutOrStdout()
	if out.Facet == "" {
		if len(out.Transitions) == 0 {
			fmt.Fprintln(w, "(該当する遷移はありません)")
			return
		}
		for _, t := range out.Transitions {
			fmt.Fprintln(w, t.ID)
		}
		return
	}

	fmt.Fprintf(w, "facet: %s\n", out.Facet)
	for _, root := range out.Roots {
		printFacetNode(w, root, 0)
	}
	if len(out.Untagged) > 0 {
		fmt.Fprintln(w, "(untagged)")
		for _, t := range out.Untagged {
			fmt.Fprintf(w, "    %s\n", t.ID)
		}
	}
}

func printFacetNode(w io.Writer, node facetNode, depth int) {
	indent := strings.Repeat("  ", depth)
	name := node.Tag.Name
	if name == "" {
		name = node.Tag.ID
	}
	fmt.Fprintf(w, "%s- %s (%s)\n", indent, node.Tag.ID, name)
	for _, t := range node.Transitions {
		fmt.Fprintf(w, "%s    %s\n", indent, t.ID)
	}
	for _, c := range node.Children {
		printFacetNode(w, c, depth+1)
	}
}
