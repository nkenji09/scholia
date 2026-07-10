package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newVocabRmCmd() *cobra.Command {
	var category string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "語彙を削除する（未参照限定・§6）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := openStore()
			if err != nil {
				return err
			}
			if category != "" {
				v, err := s.LoadVocab(id)
				if err != nil {
					return fmt.Errorf("vocab %q を読み込めません: %w", id, err)
				}
				if v.Category != category {
					return fmt.Errorf("vocab %q のカテゴリは %q です（--category %q と不一致）", id, v.Category, category)
				}
			}

			result, err := s.RemoveVocab(id)
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "vocab %s を削除しました\n", result.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "削除前のカテゴリ確認用（condition|action|effect）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "削除サマリを JSON で出力する")
	return cmd
}
