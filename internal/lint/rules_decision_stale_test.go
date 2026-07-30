package lint

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nkenji09/scholia/internal/store"
)

// staleRepo は decision-stale を「入力（git 履歴）→ 出力（findings）」の対で
// 検査するための合成 repo。実プロジェクトの履歴を一切読まないので、この規則を
// 検査していない作業が commit を積んでも結果が動かない。
type staleRepo struct {
	t   *testing.T
	dir string
}

func newStaleRepo(t *testing.T) *staleRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	r := &staleRepo{t: t, dir: t.TempDir()}
	r.git("init", "-q")
	r.git("config", "user.email", "test@example.com")
	r.git("config", "user.name", "test")
	return r
}

func (r *staleRepo) git(args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (r *staleRepo) write(relPath, body string) {
	r.t.Helper()
	p := filepath.Join(r.dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *staleRepo) writeTag(name string) {
	r.t.Helper()
	r.write(".scholia/tags/subject.x.json",
		`{"id":"subject.x","name":"`+name+`","kind":"subject"}`+"\n")
}

func (r *staleRepo) writeDecision(id, why string) {
	r.t.Helper()
	r.write(".scholia/decisions/"+id+".json",
		`{"id":"`+id+`","target":{"type":"tag","id":"subject.x"},`+
			`"at":"2026-07-30T00:00:00Z","why":"`+why+`"}`+"\n")
}

func (r *staleRepo) commit(msg string) {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "-m", msg)
}

func (r *staleRepo) emptyCommits(n int, prefix string) {
	r.t.Helper()
	for i := 0; i < n; i++ {
		r.git("commit", "-q", "--allow-empty", "-m", prefix+strconv.Itoa(i))
	}
}

// staleTargets は decision-stale が挙げた commit hash を返す。Root だけ持つ
// snapshot で足りる（checkDecisionStale が読むのは Root と Decisions のみで、
// acknowledges を張らない検査では Decisions は空でよい）。
func (r *staleRepo) staleTargets() []string {
	r.t.Helper()
	var out []string
	for _, f := range checkDecisionStale(store.Snapshot{Root: r.dir}) {
		if f.Rule != "decision-stale" {
			r.t.Fatalf("checkDecisionStale が別規則を返した: %+v", f)
		}
		out = append(out, f.Target)
	}
	return out
}

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
// 落ちない範囲: これは**窓の境界だけ**を見る。decision 同伴の判定は同ファイルの
// TestDecisionStaleDecisionAccompaniment が、acknowledges による容認は
// internal/cli の TestCLIDecisionStale が持つ。
//
// rename（R status）の除外だけは、どの検査も踏んでいない。**踏むべき差が無い**
// ためである——製品側の早期 continue は後段の「`M` で始まる status だけを数える」
// 判定と重複しており、`R100` は `M` で始まらないので分岐を消しても結果が変わらない。
// 実測: 純粋な rename（`R100\t<old>\t<new>`）を作って分岐の有無で比較したところ、
// どちらも findings 0 件だった。射程の穴ではなく、冗長な分岐である。
func TestDecisionStaleWindowBoundary(t *testing.T) {
	r := newStaleRepo(t)

	// A（新規追加）は decision-coverage の領分なので数えない。M（既存の変更）で
	// かつ decision の追加が同伴しない commit だけが decision-stale になる。
	r.writeTag("主題")
	r.commit("add record")

	r.writeTag("主題v2")
	r.commit("modify record without decision")

	got := r.staleTargets()
	if len(got) != 1 {
		t.Fatalf("レコード変更 commit（decision 非同伴）が1件出るはず: %v", got)
	}
	target := got[0]

	// 対象 commit を HEAD から深さ limit-1 まで押し下げる＝窓のちょうど内側。
	// git log -nLIMIT が返すのは深さ 0..LIMIT-1 なので、ここではまだ出る。
	r.emptyCommits(decisionStaleScanLimit-1, "pad-inside-")

	got = r.staleTargets()
	if len(got) != 1 || got[0] != target {
		t.Fatalf("深さ %d（窓のちょうど内側）ではまだ出るはず: %v", decisionStaleScanLimit-1, got)
	}

	// あと1つ積むと深さ limit＝窓の外。ここで落ちるのが設計であり、
	// 「是正された」わけでも「回帰した」わけでもない。
	r.emptyCommits(1, "pad-outside-")

	if got = r.staleTargets(); len(got) != 0 {
		t.Fatalf("深さ %d（窓の外）では消えるはず: %v", decisionStaleScanLimit, got)
	}
}

// TestDecisionStaleDecisionAccompaniment は decision-stale の中核判定——
// **レコードの変更（M）と同じ commit に decision の追加（A）が載っていれば出さない**
// ——を、同伴の有無だけが違う2つの commit の対で検査する。
//
// この面にこの検査を置く理由。dogfood ガード（internal/cli の
// TestRetrofitDogfoodAdvisories）は移動窓由来の finding を**件数もろとも数えない**
// ので、同伴判定が壊れて decision-stale が「レコードを触った commit すべて」を
// 挙げるようになっても気づけない。以前は dogfood 側の絶対件数の固定が偶然その網に
// なっていた（`addedDecision = true` を `false` に潰す変異で ack-only 14→18）。
// 件数の固定をやめた以上、**その網はこの面が明示的に持つ**。
//
// 落ちない範囲: 同伴の有無という1変数だけを動かしている。窓の境界は
// TestDecisionStaleWindowBoundary、acknowledges による容認は internal/cli の
// TestCLIDecisionStale が持つ。
func TestDecisionStaleDecisionAccompaniment(t *testing.T) {
	r := newStaleRepo(t)
	r.writeTag("主題")
	r.commit("add record")

	// (1) レコードの M と decision の A が同じ commit に載っている → 出さない。
	r.writeTag("主題v2")
	r.writeDecision("01TEST0000000000000000000A", "同伴した decision")
	r.commit("modify record WITH decision")

	if got := r.staleTargets(); len(got) != 0 {
		t.Fatalf("decision を同伴した commit は出ないはず: %v", got)
	}

	// (2) 同じ形の変更で decision だけ落とす → 出る。
	// (1) と (2) の差は「同じ commit に decision の A があるか」だけである。
	r.writeTag("主題v3")
	r.commit("modify record without decision")

	head := headHash(r)
	got := r.staleTargets()
	if len(got) != 1 || got[0] != head {
		t.Fatalf("decision 非同伴の commit %s が1件だけ出るはず: %v", head, got)
	}
}

func headHash(r *staleRepo) string {
	r.t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = r.dir
	out, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return string(out[:len(out)-1]) // 末尾の改行を落とす
}
