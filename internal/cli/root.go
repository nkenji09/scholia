// Package cli wires the cobra command tree. Commands stay thin; logic lives in internal/* (§9).
package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/store"
)

var dirFlag string

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "pmem",
		Short:         "product-memory — AI 向けコンテキスト保存支援ツール",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	cmd.PersistentFlags().StringVar(&dirFlag, "dir", "", "プロジェクトルート（既定: .pmem を上方探索）")

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newVocabCmd())
	cmd.AddCommand(newTagCmd())
	cmd.AddCommand(newTxCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newLintCmd())
	cmd.AddCommand(newDecideCmd())
	cmd.AddCommand(newRulesCmd())

	return cmd
}

// Execute is the CLI entrypoint called from cmd/pmem/main.go.
func Execute() error {
	return newRootCmd().Execute()
}

// openStore は init 以外のコマンドが .pmem を解決する共通ヘルパ。
// --dir があればそのプロジェクトルート直下の .pmem を、無ければ cwd から上方探索する（DESIGN に明記の無い実装判断）。
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
