package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newTagRmCmd() *cobra.Command {
	var force, asJSON bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "タグを削除する（既定は未参照のみ・--force で detach cascade・§6）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := openStore()
			if err != nil {
				return err
			}
			result, err := s.RemoveTag(id, force)
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "tag %s を削除しました（detached: transition %d 件・vocab %d 件・tag %d 件）\n",
				result.ID, len(result.DetachedTransitions), len(result.DetachedVocab), len(result.DetachedTags))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "参照元から detach してから削除する")
	cmd.Flags().BoolVar(&asJSON, "json", false, "削除サマリを JSON で出力する")
	return cmd
}
