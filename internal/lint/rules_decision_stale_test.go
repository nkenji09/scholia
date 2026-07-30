package lint

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nkenji09/scholia/internal/store"
)

// TestDecisionStaleWindowBoundary は decision-stale の**移動窓の境界**を、
// 生の git 履歴ではなく合成した履歴で決定的に検査する。
//
// decision-stale は直近 decisionStaleScanLimit commit だけを走査する。この
// 「窓を1つ越えたら1件落ちる」という挙動は、**固定の commit 距離を作れば
// 決定的に測れる**——実プロジェクトの履歴に任せる必要はない。任せると、この
// 規則を検査していない作業が commit を積むだけで期待値が動く（その実害と、
// dogfood 側でそれをやめた理由は internal/cli の
// TestRetrofitDogfoodAdvisories に書いた）。
//
// 検査するのは入力（git 履歴）と出力（findings）の対で、境界の**両側**を踏む:
//   - 対象 commit が HEAD から数えて窓のちょうど内側（深さ limit-1）→ 1件出る
//   - もう1つ commit を積んで窓の外（深さ limit）→ 0件になる
//
// 落ちない範囲: これは窓の境界だけを見る。decision 同伴の判定・rename 除外・
// acknowledges による容認は別の検査（internal/cli の TestCLIDecisionStale）の
// 領分で、ここでは踏んでいない。
func TestDecisionStaleWindowBoundary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	writeTag := func(name string) {
		t.Helper()
		p := filepath.Join(dir, ".scholia", "tags", "subject.x.json")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"id":"subject.x","name":"` + name + `","kind":"subject"}` + "\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-q")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "test")

	// A（新規追加）は decision-coverage の領分なので数えない。M（既存の変更）で
	// かつ decision の追加が同伴しない commit だけが decision-stale になる。
	writeTag("主題")
	git("add", "-A")
	git("commit", "-q", "-m", "add record")

	writeTag("主題v2")
	git("add", "-A")
	git("commit", "-q", "-m", "modify record without decision")

	// Root だけ持つ snapshot で足りる（checkDecisionStale が読むのは Root と
	// Decisions のみ・acknowledges は張らないので Decisions は空でよい）。
	snap := store.Snapshot{Root: dir}

	staleTargets := func() []string {
		t.Helper()
		var out []string
		for _, f := range checkDecisionStale(snap) {
			if f.Rule != "decision-stale" {
				t.Fatalf("checkDecisionStale が別規則を返した: %+v", f)
			}
			out = append(out, f.Target)
		}
		return out
	}

	got := staleTargets()
	if len(got) != 1 {
		t.Fatalf("レコード変更 commit（decision 非同伴）が1件出るはず: %v", got)
	}
	target := got[0]

	// 対象 commit を HEAD から深さ limit-1 まで押し下げる＝窓のちょうど内側。
	// git log -nLIMIT が返すのは深さ 0..LIMIT-1 なので、ここではまだ出る。
	pad := func(n int, tagMsg string) {
		t.Helper()
		for i := 0; i < n; i++ {
			git("commit", "-q", "--allow-empty", "-m", tagMsg+strconv.Itoa(i))
		}
	}
	pad(decisionStaleScanLimit-1, "pad-inside-")

	got = staleTargets()
	if len(got) != 1 || got[0] != target {
		t.Fatalf("深さ %d（窓のちょうど内側）ではまだ出るはず: %v", decisionStaleScanLimit-1, got)
	}

	// あと1つ積むと深さ limit＝窓の外。ここで落ちるのが設計であり、
	// 「是正された」わけでも「回帰した」わけでもない。
	pad(1, "pad-outside-")

	if got = staleTargets(); len(got) != 0 {
		t.Fatalf("深さ %d（窓の外）では消えるはず: %v", decisionStaleScanLimit, got)
	}
}
