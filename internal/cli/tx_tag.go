package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newTxTagCmd() *cobra.Command {
	var add, rm, set []string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "tag <id>",
		Short: "遷移にタグを付与/除去/置換する",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			setChanged := cmd.Flags().Changed("set")
			addRmChanged := len(add) > 0 || len(rm) > 0
			if !setChanged && !addRmChanged {
				return fmt.Errorf("--add/--rm か --set のいずれかを指定してください")
			}
			if setChanged && addRmChanged {
				return fmt.Errorf("--set は --add/--rm と同時に指定できません")
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			t, err := s.LoadTransition(id)
			if err != nil {
				return fmt.Errorf("transition %q を読み込めません: %w", id, err)
			}

			if setChanged {
				for _, tagID := range set {
					if !s.TagExists(tagID) {
						return fmt.Errorf("tag %q が実在しません", tagID)
					}
				}
				t.Tags = set
			} else {
				for _, tagID := range add {
					if !s.TagExists(tagID) {
						return fmt.Errorf("tag %q が実在しません", tagID)
					}
				}
				tagSet := make(map[string]bool, len(t.Tags)+len(add))
				for _, tg := range t.Tags {
					tagSet[tg] = true
				}
				for _, tg := range add {
					tagSet[tg] = true
				}
				for _, tg := range rm {
					delete(tagSet, tg)
				}
				tags := make([]string, 0, len(tagSet))
				for tg := range tagSet {
					tags = append(tags, tg)
				}
				sort.Strings(tags)
				t.Tags = tags
			}

			if err := s.SaveTransition(t); err != nil {
				return err
			}
			saved, err := s.LoadTransition(id)
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(saved)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "transition %s のタグを更新しました\n", id)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&add, "add", nil, "追加するタグ id（複数指定可）")
	cmd.Flags().StringArrayVar(&rm, "rm", nil, "除去するタグ id（複数指定可）")
	cmd.Flags().StringSliceVar(&set, "set", nil, "完全置換するタグ id（カンマ区切り）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後のレコードを JSON で出力する")
	return cmd
}
