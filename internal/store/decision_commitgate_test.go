package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/commitcheck"
	"github.com/nkenji09/scholia/internal/gittest"
	"github.com/nkenji09/scholia/internal/model"
)

// この歯止めが落とす範囲（CLAUDE.md「配線ガードの書き方」6）は
// decision_commitgate.go の冒頭に書いてある。ここはそれを実測に変える。
//
// ⚠️ **面（`decide` / `add-commit` / viewer）を1つも起こしていない。** 見ているのは
// 口そのもの——面を数える形にすると、4面目を足した誰かが配線を忘れたときに何も
// 落ちない（この repo が繰り返し落としてきた型）。

// gitT は使い捨て repo への git 呼び出しの唯一の入口（internal/gittest）を通す。
// **ここに自前の exec.Command を書かない**——background maintenance を止める設定は
// この package のどこかが gittest を import していないと効かない（gittest の doc）。
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(gittest.Run(t, dir, args...))
}

// gitBackedStore は git 管理下の store と、その HEAD の commit hash を返す。
func gitBackedStore(t *testing.T) (*Store, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("この歯止めは git を要る（実在照合そのものを見るため。skip は素通りと見分けがつかない）: %v", err)
	}
	dir := t.TempDir()
	gittest.InitRepo(t, dir)
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "seed")
	return s, gitT(t, dir, "rev-parse", "HEAD")
}

// 新規作成の口は、実在しない commit を結んだ decision を1つも作らない。
func TestCreateDecisionRejectsUnknownCommit(t *testing.T) {
	s, head := gitBackedStore(t)

	d := decision("01D1", headedWhy)
	d.Commits = []string{"0123456789abcdef0123456789abcdef01234567"}
	if _, err := s.CreateDecision(d, DecisionCreateOptions{}); err == nil {
		t.Fatal("実在しない commit を結んだ decision は作らせないべき")
	}
	if files := decisionFiles(t, s); len(files) != 0 {
		t.Fatalf("拒んだのにファイルができている: %v", files)
	}

	// 実測で保存されてしまっていた値（日本語の文字列）。
	d.Commits = []string{"これはハッシュではない"}
	_, err := s.CreateDecision(d, DecisionCreateOptions{})
	if err == nil {
		t.Fatal("commit hash の形でない値は保存させないべき")
	}
	var rej *commitcheck.RejectError
	if !asCommitReject(err, &rej) {
		t.Fatalf("commitcheck.RejectError であるべき: %T %v", err, err)
	}

	// 実在する commit なら通る。
	d.Commits = []string{head}
	if _, err := s.CreateDecision(d, DecisionCreateOptions{}); err != nil {
		t.Fatalf("実在する commit は通るべき: %v", err)
	}
}

// 更新の口も同じ歯止めを持つ。ただし見るのは**今回増えた分**だけ
// ——既に保存されている値まで見ると、無関係の更新（改名の追随など）が
// 昔から入っている値で落ちる。
func TestUpdateDecisionChecksOnlyAdditions(t *testing.T) {
	s, head := gitBackedStore(t)

	// 歯止めを迂回して、形の合わない commit を持つ既存レコードを作る
	//（この歯止めが入る前に保存された 187 件を模す）。
	legacy := decision("01D1", headedWhy)
	legacy.Commits = []string{"むかしのあたい"}
	if err := s.writeDecision(legacy); err != nil {
		t.Fatalf("直書き: %v", err)
	}

	// 既存の値はそのままに、別の欄だけを書き戻す→通る。
	legacy.Ref = "PR#1"
	if _, err := s.UpdateDecision(legacy); err != nil {
		t.Fatalf("既存の値は再検査しないはず: %v", err)
	}

	// 新しく足す hash は検査される。
	bad := legacy
	bad.Commits = append(append([]string(nil), legacy.Commits...), "0123456789abcdef0123456789abcdef01234567")
	if _, err := s.UpdateDecision(bad); err == nil {
		t.Fatal("新しく足した実在しない commit は止めるべき")
	}

	good := legacy
	good.Commits = append(append([]string(nil), legacy.Commits...), head)
	if _, err := s.UpdateDecision(good); err != nil {
		t.Fatalf("新しく足した実在する commit は通るべき: %v", err)
	}
}

// 印（applied[]）の形の検査も口で当たる。ここでも見るのは増えた分だけ。
func TestDecisionPortChecksAppliedMarks(t *testing.T) {
	s, head := gitBackedStore(t)
	const at = "2026-08-18T00:00:00Z"

	d := decision("01D1", headedWhy)
	d.Applied = []model.AppliedMark{{Kind: "adopted", At: at}}
	if _, err := s.CreateDecision(d, DecisionCreateOptions{}); err == nil {
		t.Fatal("3値でない種別は保存させないべき")
	}

	// 是正の印が持つ commit も実在照合の対象（印だけ偽物を通す抜け道を作らない）。
	d.Applied = []model.AppliedMark{{Kind: model.AppliedCorrection, At: at, Commit: "0123456789abcdef0123456789abcdef01234567"}}
	if _, err := s.CreateDecision(d, DecisionCreateOptions{}); err == nil {
		t.Fatal("印が指す実在しない commit も止めるべき")
	}

	d.Applied = []model.AppliedMark{{Kind: model.AppliedCorrection, At: at, Commit: head}}
	if _, err := s.CreateDecision(d, DecisionCreateOptions{}); err != nil {
		t.Fatalf("実在する commit を指す是正の印は通るべき: %v", err)
	}

	// 自分自身を指す印は落ちる。
	d2 := decision("01D2", headedWhy)
	d2.Applied = []model.AppliedMark{{Kind: model.AppliedConflict, At: at, Decision: "01D2"}}
	if _, err := s.CreateDecision(d2, DecisionCreateOptions{}); err == nil {
		t.Fatal("自分自身を指す印は保存させないべき")
	}
}

// git 管理下でない store では、実在は照合できない。**保存は止めない**が、
// **形の検査は残る**（決定本文の「落とせる範囲」②の射程）。
func TestDecisionPortOutsideGit(t *testing.T) {
	s := newDecisionStore(t)
	if s.CommitRepo().Managed() {
		t.Skip("この一時ディレクトリは git 管理下にある（射程の分かれ目そのものを見る検査なので、条件が違えば見ていない）")
	}

	d := decision("01D1", headedWhy)
	d.Commits = []string{"0123456789abcdef0123456789abcdef01234567"} // 形は正しい・実在は不明
	if _, err := s.CreateDecision(d, DecisionCreateOptions{}); err != nil {
		t.Fatalf("照合できないことを理由に止めてはいけない: %v", err)
	}

	d2 := decision("01D2", headedWhy)
	d2.Commits = []string{"これはハッシュではない"}
	if _, err := s.CreateDecision(d2, DecisionCreateOptions{}); err == nil {
		t.Fatal("git 管理外でも、形が違えば止めるべき")
	}
}

func asCommitReject(err error, target **commitcheck.RejectError) bool {
	for err != nil {
		if e, ok := err.(*commitcheck.RejectError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// 🔴 **口が保存する値は、同じ commit につき1つの文字列に定まる。**
//
// レビュアの再現手順（完全 hash → 短縮 hash の順に是正を打つ）をそのまま置く。
// 直す前は `commits[]` にも `applied[]` にも2件入り、**是正の件数が上振れした。**
// applied[] は追記専用なので、打ってしまうと消せない。
func TestDecisionPortNormalizesShortHash(t *testing.T) {
	s, head := gitBackedStore(t)
	const at = "2026-08-18T00:00:00Z"
	short := head[:8]

	// 1回目: 完全 hash で是正を打つ。
	d := decision("01D1", headedWhy)
	d.Commits = []string{head}
	d.Applied = []model.AppliedMark{{Kind: model.AppliedCorrection, At: at, Commit: head}}
	saved, err := s.CreateDecision(d, DecisionCreateOptions{})
	if err != nil {
		t.Fatalf("1回目: %v", err)
	}

	// 2回目: 同じ commit を**短縮 hash**で打つ（正当な入力——保存ゲートは
	// 16 進 7〜64 文字を通す）。
	next := saved
	next.Commits = append(append([]string(nil), saved.Commits...), short)
	next.Applied = append(append([]model.AppliedMark(nil), saved.Applied...),
		model.AppliedMark{Kind: model.AppliedCorrection, At: at, Commit: short})
	saved2, err := s.UpdateDecision(next)
	if err != nil {
		t.Fatalf("2回目: %v", err)
	}

	if len(saved2.Commits) != 1 || saved2.Commits[0] != head {
		t.Errorf("同じ commit は commits[] で1件のはず: %v", saved2.Commits)
	}
	if n := model.CountApplied(saved2.Applied, model.AppliedCorrection); n != 1 {
		t.Errorf("是正は1件のはず（上振れ）: %d 件 %+v", n, saved2.Applied)
	}
	// 保存されたファイルの側でも確かめる（返り値だけを見て緑にしない）。
	onDisk, err := s.LoadDecision("01D1")
	if err != nil {
		t.Fatal(err)
	}
	if len(onDisk.Commits) != 1 || len(onDisk.Applied) != 1 {
		t.Errorf("ファイルに書かれた値も1件ずつのはず: commits=%v applied=%+v", onDisk.Commits, onDisk.Applied)
	}
}

// 逆順（短縮を先に打ってから完全 hash を打つ）でも1件に畳む。
func TestDecisionPortNormalizesShortHashReverseOrder(t *testing.T) {
	s, head := gitBackedStore(t)
	short := head[:8]

	d := decision("01D1", headedWhy)
	d.Commits = []string{short}
	saved, err := s.CreateDecision(d, DecisionCreateOptions{})
	if err != nil {
		t.Fatalf("1回目: %v", err)
	}
	// ⚠️ 1回目に渡した短縮 hash は**完全 hash へ寄って保存される**。
	if len(saved.Commits) != 1 || saved.Commits[0] != head {
		t.Fatalf("新しく足す値は完全 hash へ寄るはず: %v", saved.Commits)
	}

	next := saved
	next.Commits = append(append([]string(nil), saved.Commits...), head)
	saved2, err := s.UpdateDecision(next)
	if err != nil {
		t.Fatalf("2回目: %v", err)
	}
	if len(saved2.Commits) != 1 {
		t.Errorf("同じ commit は1件のはず: %v", saved2.Commits)
	}
}

// 口が返すのは「実際に保存された値」であること（渡した値ではない）。
// 面がこれを使わずに渡した側を出力すると、画面と真実の源がずれる。
func TestDecisionPortReturnsSavedValue(t *testing.T) {
	s, head := gitBackedStore(t)
	d := decision("01D1", headedWhy)
	d.Commits = []string{head[:10]}
	saved, err := s.CreateDecision(d, DecisionCreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Commits[0] != head {
		t.Fatalf("返り値は保存後の値であるべき: %v", saved.Commits)
	}
	if d.Commits[0] != head[:10] {
		t.Fatalf("呼び出し元に渡した値は書き換わらないはず（値渡し）: %v", d.Commits)
	}
	onDisk, err := s.LoadDecision("01D1")
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Commits[0] != head {
		t.Fatalf("ファイルの値と返り値が一致するべき: %v", onDisk.Commits)
	}
}
