package commitcheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/gittest"
)

// 形の判定は git を要らない純関数（CLAUDE.md「配線ガードの書き方」1）。
//
// **落ちる:** 16 進でない文字を含む・短すぎる・長すぎる・空。
// **落ちない:** 「その hash がこのリポジトリに在るか」——それは Repo.Verify の側。
func TestLooksLikeHash(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"0123456", true}, // 短縮長の下限（7）
		{"0123456789abcdef0123456789abcdef01234567", true}, // SHA-1 完全長
		{strings.Repeat("a", 64), true},                    // SHA-256 完全長
		{"ABCDEF0", true},                                  // 大文字も git は受ける
		{"012345", false},                                  // 6 文字は短すぎる
		{strings.Repeat("a", 65), false},                   // 65 文字は長すぎる
		{"", false},
		{"これはハッシュではない", false}, // 実測で保存されてしまっていた値
		{"HEAD", false},        // 解決はするが、後から指す先が変わる
		{"main", false},
		{"abcdefg", false}, // g は 16 進にない
		{"abc 123", false},
		{"-abc1234", false},
	}
	for _, c := range cases {
		if got := LooksLikeHash(c.in); got != c.want {
			t.Errorf("LooksLikeHash(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// 結論の組み立ても純関数。**git が無いときに「形が違う」を素通りさせない**のが
// ここの要点で、2つの検査を1つにまとめるとその枝が消える。
func TestClassify(t *testing.T) {
	cases := []struct {
		name       string
		hash       string
		gitManaged bool
		resolved   bool
		want       Verdict
	}{
		{"git 管理下・実在する", "abc1234", true, true, VerdictExists},
		{"git 管理下・実在しない", "abc1234", true, false, VerdictMissing},
		{"git 管理外・形は合っている", "abc1234", false, false, VerdictUnverifiable},
		{"git 管理外でも形が違えば落ちる", "これはハッシュではない", false, false, VerdictMalformed},
		{"git 管理下でも形が違えば落ちる", "これはハッシュではない", true, true, VerdictMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Classify(c.hash, c.gitManaged, c.resolved); got != c.want {
				t.Fatalf("Classify(%q, %v, %v) = %s, want %s", c.hash, c.gitManaged, c.resolved, got, c.want)
			}
		})
	}
}

// 保存を止めるのは malformed と missing だけ。**unverifiable では止めない**
// ——照合できないことは、その hash が偽物であることの証拠ではない。
func TestVerdictRejects(t *testing.T) {
	want := map[Verdict]bool{
		VerdictMalformed:    true,
		VerdictMissing:      true,
		VerdictUnverifiable: false,
		VerdictExists:       false,
	}
	for v, w := range want {
		if got := v.Rejects(); got != w {
			t.Errorf("%s.Rejects() = %v, want %v", v, got, w)
		}
	}
}

// gitT は使い捨て repo への git 呼び出しの唯一の入口（internal/gittest）を通す。
// **ここに自前の exec.Command を書かない**——background maintenance を止める設定は
// この package のどこかが gittest を import していないと効かない（gittest の doc）。
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(gittest.Run(t, dir, args...))
}

// seedRepo は commit を1つ持つ git リポジトリを作り、その hash を返す。
func seedRepo(t *testing.T) (dir, head string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("この検査は git を要る（実在照合そのものを見るため。skip は素通りと見分けがつかない）: %v", err)
	}
	dir = t.TempDir()
	gittest.InitRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "seed")
	return dir, gitT(t, dir, "rev-parse", "HEAD")
}

func TestRepoVerify_InGitRepo(t *testing.T) {
	dir, head := seedRepo(t)
	repo := Open(dir)
	if !repo.Managed() {
		t.Fatal("git 管理下として解決するべき")
	}

	if got := repo.Verify(head); got != VerdictExists {
		t.Errorf("実在する hash: got %s", got)
	}
	if got := repo.Verify(head[:7]); got != VerdictExists {
		t.Errorf("短縮 hash も解決するべき: got %s", got)
	}
	// 実在しない（形は正しい）hash。
	missing := "0123456789abcdef0123456789abcdef01234567"
	if got := repo.Verify(missing); got != VerdictMissing {
		t.Errorf("実在しない hash: got %s", got)
	}
	// 実測で保存されてしまっていた値。
	if got := repo.Verify("これはハッシュではない"); got != VerdictMalformed {
		t.Errorf("hash の形でない値: got %s", got)
	}
	// ⚠️ ブランチ名・HEAD は git では解決するが、保存されるのは動かない値で
	// なければならないので通さない。
	for _, ref := range []string{"HEAD", "main", "master"} {
		if got := repo.Verify(ref); got != VerdictMalformed {
			t.Errorf("%q は通してはいけない: got %s", ref, got)
		}
	}
}

// tree の hash（commit ではない）は通さない——`^{commit}` で peel させている
// ことが効いていることの実測。
func TestRepoVerify_TreeHashIsNotACommit(t *testing.T) {
	dir, head := seedRepo(t)
	tree := gitT(t, dir, "rev-parse", head+"^{tree}")
	repo := Open(dir)
	if got := repo.Verify(tree); got != VerdictMissing {
		t.Fatalf("tree の hash を commit として通してはいけない: got %s", got)
	}
}

// git 管理下でないとき、実在は照合できない。**それでも形は効く。**
//
// ⚠️ **skip で逃げない。** 一時ディレクトリがたまたま git repo の中かどうかに
// 結果を預けると、この枝は環境次第で一度も走らなくなる（素通りと見分けが
// つかない）。Open が解決に失敗したときに返すのと同じゼロ値の Repo を直に組んで、
// **必ず走る形**にする。
func TestRepoVerify_Unmanaged(t *testing.T) {
	repo := Repo{}
	if repo.Managed() {
		t.Fatal("ゼロ値の Repo は git 管理外であるべき（Open が解決に失敗したときの返り値）")
	}
	if got := repo.Verify("0123456789abcdef0123456789abcdef01234567"); got != VerdictUnverifiable {
		t.Errorf("git 管理外では照合できない（保存は止めない）: got %s", got)
	}
	if got := repo.Verify("これはハッシュではない"); got != VerdictMalformed {
		t.Errorf("git 管理外でも形は効く: got %s", got)
	}
	// 止めるものが無ければ Check は通す（unverifiable では止めない）。
	if err := repo.Check([]string{"0123456789abcdef0123456789abcdef01234567"}); err != nil {
		t.Errorf("照合できないことを理由に止めてはいけない: %v", err)
	}
	if err := repo.Check([]string{"これはハッシュではない"}); err == nil {
		t.Error("git 管理外でも、形が違えば止めるべき")
	}
}

// Open は git 管理下のディレクトリを managed=true として解決する（解決そのものは
// 単位BC の activity.ResolveGitContext に任せている——ここでは配線を実測する）。
func TestOpen_ResolvesGitRepo(t *testing.T) {
	dir, _ := seedRepo(t)
	if !Open(dir).Managed() {
		t.Fatal("git 管理下として解決するべき")
	}
	sub := filepath.Join(dir, "nested", "deeper")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if !Open(sub).Managed() {
		t.Fatal("リポジトリ根より下のディレクトリでも解決するべき（固定接頭辞で判定していないこと）")
	}
}

// Check は「止めるべきものが1つでもあれば」止める。文言には**どの hash が
// なぜ**止まったかが出る（出さないと、書き手は直しようがない）。
func TestRepoCheck(t *testing.T) {
	dir, head := seedRepo(t)
	repo := Open(dir)

	if err := repo.Check([]string{head}); err != nil {
		t.Fatalf("実在する hash だけなら通るべき: %v", err)
	}
	if err := repo.Check(nil); err != nil {
		t.Fatalf("空なら通るべき: %v", err)
	}

	err := repo.Check([]string{head, "これはハッシュではない"})
	if err == nil {
		t.Fatal("1つでも止めるべきものがあれば止まるべき")
	}
	var rej *RejectError
	if !asReject(err, &rej) {
		t.Fatalf("RejectError であるべき: %T", err)
	}
	if !strings.Contains(err.Error(), "これはハッシュではない") {
		t.Errorf("どの値が止まったかが文言に無い: %s", err)
	}

	err = repo.Check([]string{"0123456789abcdef0123456789abcdef01234567"})
	if err == nil {
		t.Fatal("実在しない hash は止めるべき")
	}
	if !strings.Contains(err.Error(), "実在しません") {
		t.Errorf("なぜ止まったかが文言に無い: %s", err)
	}
}

func asReject(err error, target **RejectError) bool {
	e, ok := err.(*RejectError)
	if ok {
		*target = e
	}
	return ok
}

// 🔴 **16 進の名前を持つブランチ/タグは通さない**（クリーンルームレビュー 指摘①）。
//
// git は `<名前>^{commit}` を **ref 優先**で解決するので、`c8d45c0` という名前の
// ブランチが在ると、**打った人が指した commit ではなくブランチの先が exit 0 で返る**
// （警告も出ない）。それをそのまま保存すると、**追記専用のフィールドに別の commit が
// 焼き付いて後から消せない。**
//
// **落ちる:** ref として解決されたとき（前方一致が破れる）。
// **落ちない:** その ref がたまたま自分の名前を接頭辞に持つ commit を指しているとき
// ——そのとき保存される値は「短縮 hash として解決した結果」と同じなので害が無い。
func TestRepoResolve_RejectsHexNamedRef(t *testing.T) {
	dir, first := seedRepo(t)
	// 2つ目の commit を作る。
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "second")
	second := gitT(t, dir, "rev-parse", "HEAD")
	if first == second {
		t.Fatal("2つ目の commit が作れていない")
	}

	// **1つ目の commit の短縮 hash を名前にしたブランチ**を、2つ目へ向ける。
	shortOfFirst := first[:7]
	gitT(t, dir, "branch", shortOfFirst, second)

	repo := Open(dir)

	// git 自身は ref を先に解決する（この検査が何を相手にしているかの実測）。
	if got := gitT(t, dir, "rev-parse", "--verify", "--quiet", shortOfFirst+"^{commit}"); got != second {
		t.Fatalf("前提が崩れている（git が ref を優先していない）: %s", got)
	}

	// 🔴 それでも scholia は通さない。
	res := repo.Resolve(shortOfFirst)
	if res.Verdict != VerdictMissing {
		t.Fatalf("16 進の名前を持つ ref は通してはいけない: %+v", res)
	}
	if res.Canonical != "" {
		t.Fatalf("通さないのに完全 hash を返してはいけない: %+v", res)
	}
	if err := repo.Check([]string{shortOfFirst}); err == nil {
		t.Fatal("保存前に止めるべき")
	}

	// ⚠️ 同じ名前でも、**その ref が自分の名前を接頭辞に持つ commit を指しているなら**
	// 通る（保存される値は短縮 hash として解決した結果と同じ＝害が無い）。
	shortOfSecond := second[:7]
	gitT(t, dir, "branch", shortOfSecond, second)
	if res := repo.Resolve(shortOfSecond); res.Verdict != VerdictExists || res.Canonical != second {
		t.Fatalf("接頭辞が一致するなら通してよい: %+v", res)
	}

	// 大文字で打っても同じ（git は大文字 16 進も受ける・出力は小文字）。
	if res := repo.Resolve(strings.ToUpper(second[:10])); res.Verdict != VerdictExists || res.Canonical != second {
		t.Fatalf("大文字の短縮 hash は通るべき: %+v", res)
	}
}

// タグでも同じ（annotated tag は `^{commit}` で peel されるので、なお通しやすい）。
func TestRepoResolve_RejectsHexNamedTag(t *testing.T) {
	dir, first := seedRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", "second")
	second := gitT(t, dir, "rev-parse", "HEAD")

	gitT(t, dir, "tag", "-a", first[:8], "-m", "annotated", second)

	if res := Open(dir).Resolve(first[:8]); res.Verdict != VerdictMissing {
		t.Fatalf("16 進の名前を持つ tag は通してはいけない: %+v", res)
	}
}

// 止める文言は、同じ値について1回だけ言う。
// `add-commit --kind correction` は commits[] と applied[] の両方に同じ hash を
// 載せるので、畳まないと同じ文が2回出る（実測で出た）。
func TestRejectErrorSaysEachValueOnce(t *testing.T) {
	dir, _ := seedRepo(t)
	repo := Open(dir)
	err := repo.Check([]string{"これはハッシュではない", "これはハッシュではない"})
	if err == nil {
		t.Fatal("止めるべき")
	}
	if n := strings.Count(err.Error(), "これはハッシュではない"); n != 1 {
		t.Fatalf("同じ値は1回だけ言うべき（%d 回出た）: %s", n, err)
	}
}
