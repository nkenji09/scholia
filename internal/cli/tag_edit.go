package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/lint"
)

func newTagEditCmd() *cobra.Command {
	var name, kind, desc, descFile, color, ref string
	var editDesc, total bool
	var parents []string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "タグの指定フィールドのみ更新する",
		Long: "タグの指定フィールドのみ更新する。\n\n" +
			"--total は kind=axis タグ向け（#39・§3.4）: 軸の値のうち必ず1つが真であるべきかを宣言し、" +
			"true にすると値の condition がどの transition の given にも現れない場合に抜け(L-total)として検出される。" +
			"片方の値が本質的に no-op（対応する transition が無いのが正しい）な2値軸を total にすると、" +
			"no-op 側が偽の抜けとして出る——そのような軸は非 total にするか、値を分割して表すこと（#40・DESIGN §3.4）。",
		Args: cobra.ExactArgs(1),
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
			descValue, descChanged, err := descSource{
				direct:    desc,
				directSet: cmd.Flags().Changed("desc"),
				file:      descFile,
				edit:      editDesc,
			}.resolve()
			if err != nil {
				return err
			}
			if descChanged {
				t.Description = descValue
			}
			if cmd.Flags().Changed("color") {
				t.Color = color
			}
			if cmd.Flags().Changed("ref") {
				t.Ref = ref
			}
			if cmd.Flags().Changed("total") {
				t.Total = total
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
	cmd.Flags().StringVar(&desc, "desc", "", "説明（空指定で解除・--desc-file/--edit と排他）")
	cmd.Flags().StringVar(&descFile, "desc-file", "", "説明をファイルから読み込む（--desc/--edit と排他）")
	cmd.Flags().BoolVar(&editDesc, "edit", false, "$EDITOR で説明を入力する（--desc/--desc-file と排他）")
	cmd.Flags().StringVar(&color, "color", "", "表示色（空指定で解除）")
	cmd.Flags().StringVar(&ref, "ref", "", "参照 URL（空指定で解除）")
	cmd.Flags().BoolVar(&total, "total", false, "kind=axis タグ向け: 軸の値のうち必ず1つが真であるべきか（#39・§3.4）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後のレコードを JSON で出力する")
	return cmd
}
