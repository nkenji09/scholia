// Package cli wires the cobra command tree. Commands stay thin; logic lives in internal/* (§9).
package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/store"
)

var dirFlag string

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "scholia",
		Short:         "scholia — AI 向けコンテキスト保存支援ツール",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	cmd.PersistentFlags().StringVar(&dirFlag, "dir", "", "プロジェクトルート（既定: .scholia を上方探索）")

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newKindCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newVocabCmd())
	cmd.AddCommand(newTagCmd())
	cmd.AddCommand(newTxCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newLintCmd())
	cmd.AddCommand(newDecideCmd())
	cmd.AddCommand(newDecisionCmd())
	cmd.AddCommand(newReviewCmd())
	cmd.AddCommand(newRulesCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newSpecCmd())
	cmd.AddCommand(newFlowCmd())
	cmd.AddCommand(newGapsCmd())
	cmd.AddCommand(newDiffCmd())
	cmd.AddCommand(newSearchCmd())
	cmd.AddCommand(newViewCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newSkillsCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newRefsCmd())
	cmd.AddCommand(newUpdateCmd())

	return cmd
}

// Execute is the CLI entrypoint called from cmd/scholia/main.go.
func Execute() error {
	return newRootCmd().Execute()
}

// openStore は init 以外のコマンドが .scholia を解決する共通ヘルパ。
// --dir があればそのプロジェクトルート直下の .scholia を、無ければ cwd から上方探索する（DESIGN に明記の無い実装判断）。
func openStore() (*store.Store, error) {
	if dirFlag != "" {
		return store.Open(dirFlag)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return store.Discover(cwd)
}
