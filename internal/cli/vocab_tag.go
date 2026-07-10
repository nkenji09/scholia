package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// newVocabTagCmd は語彙にタグを付与/除去する（§3.3・実効タグの語彙経路）。
// Phase 1 のスコープ表には無いが、これが無いと `vocab tag` 経路の実効タグ
// （§3.7 の「参照している vocab のタグ」）を CLI から作れず、実効タグの意味が
// 半分しか成立しない（handoff の指示どおり本スコープに含める）。
func newVocabTagCmd() *cobra.Command {
	var add, rm []string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "tag <id>",
		Short: "語彙にタグを付与/除去する（遷移はこのタグを継承する・§3.7）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if len(add) == 0 && len(rm) == 0 {
				return fmt.Errorf("--add か --rm の少なくとも一方を指定してください")
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			v, err := s.LoadVocab(id)
			if err != nil {
				return fmt.Errorf("vocab %q を読み込めません: %w", id, err)
			}
			for _, tagID := range add {
				if !s.TagExists(tagID) {
					return fmt.Errorf("tag %q が実在しません", tagID)
				}
			}

			set := make(map[string]bool, len(v.Tags)+len(add))
			for _, t := range v.Tags {
				set[t] = true
			}
			for _, t := range add {
				set[t] = true
			}
			for _, t := range rm {
				delete(set, t)
			}
			tags := make([]string, 0, len(set))
			for t := range set {
				tags = append(tags, t)
			}
			sort.Strings(tags)
			v.Tags = tags

			if err := s.SaveVocab(v); err != nil {
				return err
			}
			saved, err := s.LoadVocab(id)
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(saved)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "vocab %s のタグを更新しました\n", id)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&add, "add", nil, "追加するタグ id（複数指定可）")
	cmd.Flags().StringArrayVar(&rm, "rm", nil, "除去するタグ id（複数指定可）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後のレコードを JSON で出力する")
	return cmd
}
