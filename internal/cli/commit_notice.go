package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/commitcheck"
)

// writeCommitVerifyNotice は「結ぶ commit の実在を照合できなかった」ことを
// 名乗る（decision 01M09FHEQH7PVZ2BTKGXY5YMNN・commitcheck の package doc）。
//
// git 管理下でないストア（git が PATH に無い場合を含む）では、hash の**形**は
// 検査できても**実在**は照合できない。ここで黙ると、呼んだ側は検査が走ったと
// 読む——`scholia activity` が浅い clone で数を出さずに名乗るのと同じ理由で、
// 保存はしたうえで照合していないことを出す。
//
// hashes が 0 のときは何も出さない（照合する相手が無い呼び出しで注記だけ出ると、
// 何について言われているのか読めない）。
func writeCommitVerifyNotice(cmd *cobra.Command, repo commitcheck.Repo, hashes int) {
	if hashes == 0 || repo.Managed() {
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(),
		"⚠️ commit の実在は照合していません（git 管理下ではありません。形が commit hash であることだけを確かめました）。")
}
