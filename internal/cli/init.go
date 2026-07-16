package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/store"
)

func newInitCmd() *cobra.Command {
	var asJSON bool
	var noGitignore bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: ".scholia/ を作成する（冪等）",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := dirFlag
			if root == "" {
				root = "."
			}
			s, err := store.InitWithOptions(root, store.InitOptions{SkipGitignore: noGitignore})
			if err != nil {
				return err
			}
			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(cfg)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s を作成しました\n", s.Dir)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "config.json を JSON で出力する")
	cmd.Flags().BoolVar(&noGitignore, "no-gitignore", false, ".gitignore への .scholia/index.db 追記をスキップする")
	return cmd
}
