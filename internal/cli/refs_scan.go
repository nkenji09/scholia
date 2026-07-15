package cli

import (
	"encoding/json"
	"sort"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/refs"
)

func newRefsScanCmd() *cobra.Command {
	var id string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "ソース中の pmem id 出現を一覧する（健全性・棚卸し用・.pmem は変更しない）",
		Long: "ソースコード中に現れる pmem id（vocab/tag/transition）の出現を境界安全に一覧する。\n" +
			"--id を指定するとその id だけを走査、省略すると .pmem/ 内の全 id を走査する。",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			var ids []string
			if id != "" {
				ids = []string{id}
			} else {
				snap, err := s.LoadAll()
				if err != nil {
					return err
				}
				for _, v := range snap.Vocab {
					ids = append(ids, v.ID)
				}
				for _, t := range snap.Tags {
					ids = append(ids, t.ID)
				}
				for _, tx := range snap.Transitions {
					ids = append(ids, tx.ID)
				}
				sort.Strings(ids)
			}

			report, err := refs.ScanIDs(projectRoot(s), ids)
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			printRenameRefsReport(cmd, &report, false)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "この id だけを走査する（省略時は全 id）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "結果を JSON で出力する")
	return cmd
}
