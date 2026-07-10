package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/lint"
)

func newTagEditCmd() *cobra.Command {
	var name, kind, desc, color, ref string
	var parents []string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "タグの指定フィールドのみ更新する",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := openStore()
			if err != nil {
				return err
			}
			t, err := s.LoadTag(id)
			if err != nil {
				return fmt.Errorf("tag %q を読み込めません: %w", id, err)
			}

			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("name") {
				if name == "" {
					return fmt.Errorf("--name を空にはできません")
				}
				t.Name = name
			}
			if cmd.Flags().Changed("kind") {
				if kind != "" && !containsStr(snap.Config.TagKinds, kind) {
					return fmt.Errorf("kind %q は config.tagKinds に未宣言です", kind)
				}
				t.Kind = kind
			}
			if cmd.Flags().Changed("parent") {
				for _, p := range parents {
					if p == id {
						return fmt.Errorf("tag %q は自分自身を parent にできません", id)
					}
					if !s.TagExists(p) {
						return fmt.Errorf("parent %q が実在しません", p)
					}
				}
				parentGraph := make(map[string][]string, len(snap.Tags))
				for _, tg := range snap.Tags {
					parentGraph[tg.ID] = tg.ParentIDs
				}
				parentGraph[id] = parents
				for _, cycled := range lint.CycleMembers(parentGraph) {
					if cycled == id {
						return fmt.Errorf("tag %q の parentIds が循環を作ります", id)
					}
				}
				t.ParentIDs = parents
			}
			if cmd.Flags().Changed("desc") {
				t.Description = desc
			}
			if cmd.Flags().Changed("color") {
				t.Color = color
			}
			if cmd.Flags().Changed("ref") {
				t.Ref = ref
			}

			if err := s.SaveTag(t); err != nil {
				return err
			}
			saved, err := s.LoadTag(id)
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(saved)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "tag %s を更新しました\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "表示名")
	cmd.Flags().StringVar(&kind, "kind", "", "kind（config.tagKinds の宣言集合に含まれる必要がある。空指定で解除）")
	cmd.Flags().StringArrayVar(&parents, "parent", nil, "親タグ id（複数指定可・完全置換・循環拒否）")
	cmd.Flags().StringVar(&desc, "desc", "", "説明（空指定で解除）")
	cmd.Flags().StringVar(&color, "color", "", "表示色（空指定で解除）")
	cmd.Flags().StringVar(&ref, "ref", "", "参照 URL（空指定で解除）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後のレコードを JSON で出力する")
	return cmd
}
