package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newTxRmCmd() *cobra.Command {
	var why string
	var force, asJSON bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "遷移を削除する（破壊的・その遷移を target とする decisions も道連れ削除・§6）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if why == "" {
				return fmt.Errorf("--why は必須です（破壊的操作の理由）")
			}
			if !force {
				return fmt.Errorf("--force は必須です（破壊的操作の確認）")
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			result, err := s.RemoveTransition(id, why)
			if err != nil {
				return err
			}

			for _, dID := range result.RemovedDecisions {
				fmt.Fprintf(cmd.ErrOrStderr(), "decision %s を道連れ削除しました（why: %s）\n", dID, why)
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "transition %s を削除しました（道連れ decision: %d 件・why: %s）\n",
				result.ID, len(result.RemovedDecisions), why)
			return nil
		},
	}
	cmd.Flags().StringVar(&why, "why", "", "削除理由（必須）")
	cmd.Flags().BoolVar(&force, "force", false, "破壊的操作の確認（必須）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "削除サマリを JSON で出力する")
	return cmd
}
