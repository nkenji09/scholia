package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/model"
)

func newVocabAddCmd() *cobra.Command {
	var label, kind, owner, description string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "add <condition|action|effect> <id>",
		Short: "語彙を 1 件追加する",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			category, id := args[0], args[1]
			if category != model.CategoryCondition && category != model.CategoryAction && category != model.CategoryEffect {
				return fmt.Errorf("category は condition|action|effect のいずれかである必要があります（実際は %q）", category)
			}
			if label == "" {
				return fmt.Errorf("--label は必須です")
			}
			if owner != "" && category != model.CategoryEffect {
				return fmt.Errorf("--owner は effect カテゴリでのみ指定できます")
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			if s.VocabExists(id) {
				return fmt.Errorf("vocab %q は既に存在します", id)
			}

			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}
			if kind != "" && !containsStr(cfg.KindsFor(category), kind) {
				return fmt.Errorf("kind %q は config.kinds.%s に未宣言です", kind, category)
			}

			v := model.VocabEntry{ID: id, Category: category, Label: label, Kind: kind, Owner: owner, Description: description}
			if err := s.SaveVocab(v); err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(v)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "vocab %s を作成しました\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "表示ラベル（必須）")
	cmd.Flags().StringVar(&kind, "kind", "", "kind（config.kinds の宣言集合に含まれる必要がある）")
	cmd.Flags().StringVar(&owner, "owner", "", "効果を起こす主体（effect のみ）")
	cmd.Flags().StringVar(&description, "description", "", "説明（markdown・任意）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "作成したレコードを JSON で出力する")
	return cmd
}

func containsStr(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
