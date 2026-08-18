// Package gittest は、使い捨ての git repo を作るテスト fixture が必ず通る
// 唯一の入口である。
//
// `git commit` は毎回「`git maintenance run --auto --detach` を起動するか」を
// 検討する。git 2.54（CI の ubuntu-24.04 ランナーが積む版）ではそこで新設された
// geometric-repack タスクが detach のまま走り、fixture がまだ読み書きしている
// repo から到達可能な commit オブジェクトを削ることが実測されている
// （症状: `git log` が exit 128、または t.TempDir() の片付け時に
// "directory not empty"）。`gc.auto=0` では止まらず、`maintenance.auto=false`
// だけが効く（git 2.54.0 の Linux コンテナで実測: 既定値のまま本物のテストを
// 100 回回すと 10 回落ち、`maintenance.auto=false` を足すと 100 回中 0 回に
// なる）。git 2.47.3・macOS の git 2.50.1 には原因の geometric-repack タスク
// 自体が無いため、この2つで緑が出ても実証にならない。
//
// この repo では同じ型のガードの抜けが既に3度起きている
// （CLAUDE.md「配線ガードの書き方」5）。だから対策は「各 fixture に1行足す」
// ではなく、二段で持つ:
//
//  1. InitRepo を、この module の全 fixture が repo を作るときの唯一の
//     call site にする。
//  2. 加えて、この package の init() が同じ設定をプロセスの環境変数として
//     注入する。exec.Command は既定で親（＝テストバイナリ）の環境を継承
//     するので、同じ package 内の別の fixture が InitRepo を経由せず生の
//     exec.Command("git", ...) で repo を作っていても、その package のどこか
//     一箇所が gittest を import してさえいれば自動的に効く。
//
// 効かない範囲: gittest を一度も import しない新しい package が、独自の
// git repo fixture を書いたとき。そのときは InitRepo の呼び出し（＝この
// package の import）を手で足す必要がある——ここは原理的に防げない
// （CLAUDE.md「配線ガードの書き方」2 と同型: import しないコードを検査で
// 捕まえる方法は無い）。
package gittest

import (
	"os"
	"os/exec"
	"testing"
)

func init() {
	// GIT_CONFIG_COUNT/KEY_n/VALUE_n（git 2.31+）は設定ファイルを介さない
	// config 注入で、`git -c` と同じ優先度を持つ。以後このプロセスから
	// 起動する全ての git 呼び出しに及ぶ。
	os.Setenv("GIT_CONFIG_COUNT", "1")
	os.Setenv("GIT_CONFIG_KEY_0", "maintenance.auto")
	os.Setenv("GIT_CONFIG_VALUE_0", "false")
}

// InitRepo は dir で `git init` し、固定の test identity を設定する。
func InitRepo(t *testing.T, dir string) {
	t.Helper()
	Run(t, dir, "init", "-q")
	Run(t, dir, "config", "user.email", "test@example.com")
	Run(t, dir, "config", "user.name", "test")
}

// Run は dir で `git <args...>` を実行し、失敗したら t.Fatalf する。
// 戻り値は結合された stdout+stderr。
func Run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
