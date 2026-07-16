// Command scholia is the CLI entrypoint.
package main

import (
	"os"

	"github.com/nkenji09/scholia/internal/cli"
)

func main() {
	// cobra はエラーメッセージ自体を標準エラーに出力するので、ここでは終了コードだけ制御する。
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
