package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
)

func newTagCreateCmd() *cobra.Command {
	var name, kind, desc, descFile, color, ref string
	var editDesc, total bool
	var parents []string
	var asJSON bool
	var gate *gateFlags
	cmd := &cobra.Command{
		Use:   "create <id>",
		Short: "タグを 1 件作成する",
		Long: "タグを 1 件作成する。\n\n" +
			"--total は kind=axis タグ向け（#39・§3.4）: 軸の値のうち必ず1つが真であるべきかを宣言し、" +
			"true にすると値の condition がどの transition の given にも現れない場合に抜け(L-total)として検出される。" +
			"片方の値が本質的に no-op（対応する transition が無いのが正しい）な2値軸を total にすると、" +
			"no-op 側が偽の抜けとして出る——そのような軸は非 total にするか、値を分割して表すこと（#40・DESIGN §3.4）。",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if name == "" {
				return fmt.Errorf("--name は必須です")
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			if s.TagExists(id) {
				return fmt.Errorf("tag %q は既に存在します", id)
			}

			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			if kind != "" {
				if !containsStr(snap.Config.TagKinds, kind) {
					return fmt.Errorf("kind %q は config.tagKinds に未宣言です", kind)
				}
			} else {
				switch len(snap.Config.TagKinds) {
				case 0:
					// 退化した config（tagKinds 未宣言）: 既定できないので空を許容する。lint が後で警告する。
				case 1:
					kind = snap.Config.TagKinds[0]
				default:
					return fmt.Errorf("tagKind が複数あるため --kind が必須です: %v", snap.Config.TagKinds)
				}
			}

			for _, p := range parents {
				if !s.TagExists(p) {
					return fmt.Errorf("parent %q が実在しません", p)
				}
			}

			parentGraph := make(map[string][]string, len(snap.Tags)+1)
			for _, t := range snap.Tags {
				parentGraph[t.ID] = t.ParentIDs
			}
			parentGraph[id] = parents
			for _, cycled := range lint.CycleMembers(parentGraph) {
				if cycled == id {
					return fmt.Errorf("tag %q の parentIds が循環を作ります", id)
				}
			}

			descValue, _, err := descSource{
				direct:    desc,
				directSet: cmd.Flags().Changed("desc"),
				file:      descFile,
				edit:      editDesc,
			}.resolve()
			if err != nil {
				return err
			}

			t := model.Tag{
				ID:          id,
				Name:        name,
				Kind:        kind,
				ParentIDs:   parents,
				Description: descValue,
				Color:       color,
				Ref:         ref,
				Total:       total,
			}
			// 書き込みゲート二層（#45 U3）: total-kind-mismatch・id-policy は
			// reject（保存せず exit 1）。desc への advisory は保存後に表示。
			advisories, allowed, gateErr := runWriteGate(cmd, snap, lint.WriteOp{Tag: &t, IsNew: true}, gate)
			if gateErr != nil {
				return gateErr
			}
			if err := s.SaveTag(t); err != nil {
				return err
			}

			if asJSON {
				return emitWriteJSON(cmd, t, advisories, allowed, false)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "tag %s を作成しました\n", id)
			printWriteGateText(cmd, allowed, advisories)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "表示名（必須）")
	cmd.Flags().StringVar(&kind, "kind", "", "kind（config.tagKinds の宣言集合に含まれる必要がある）")
	cmd.Flags().StringArrayVar(&parents, "parent", nil, "親タグ id（複数指定可）")
	cmd.Flags().StringVar(&desc, "desc", "", "説明（--desc-file/--edit と排他）")
	cmd.Flags().StringVar(&descFile, "desc-file", "", "説明をファイルから読み込む（--desc/--edit と排他）")
	cmd.Flags().BoolVar(&editDesc, "edit", false, "$EDITOR で説明を入力する（--desc/--desc-file と排他）")
	cmd.Flags().StringVar(&color, "color", "", "表示色")
	cmd.Flags().StringVar(&ref, "ref", "", "参照 URL")
	cmd.Flags().BoolVar(&total, "total", false, "kind=axis タグ向け: 軸の値のうち必ず1つが真であるべきか（#39・§3.4）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "作成したレコードを応答封筒 { record, advisories } の JSON で出力する")
	gate = addGateAllowFlags(cmd)
	return cmd
}
