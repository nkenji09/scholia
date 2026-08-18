package lint

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// correction-changes-records を「入力（git 履歴と印）→ 出力（findings）」の対で
// 検査する。実プロジェクトの履歴を一切読まないので、この規則を検査していない
// 作業が commit を積んでも結果が動かない。
//
// # ここが落とす範囲（CLAUDE.md「配線ガードの書き方」6）
//
// **落ちる:** 是正の印が付いた commit が `.scholia/` 配下を変更していること。
// リポジトリ根より下にストアがある形（monorepo）でも落ちる。
//
// **落ちない:** 「本当に実装が間違っていたか」・打ち忘れ・矛盾と却下の印
// （規則の doc comment に列挙してある）。
type correctionRepo struct {
	t   *testing.T
	dir string
}

func newCorrectionRepo(t *testing.T) *correctionRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("この歯止めは git を要る（commit の中身を見るため。skip は素通りと見分けがつかない）: %v", err)
	}
	r := &correctionRepo{t: t, dir: t.TempDir()}
	r.git("init", "-q")
	r.git("config", "user.email", "guard@example.invalid")
	r.git("config", "user.name", "guard")
	return r
}

func (r *correctionRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *correctionRepo) write(relPath, body string) {
	r.t.Helper()
	p := filepath.Join(r.dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// commitAll は全変更を1つの commit にして hash を返す。
func (r *correctionRepo) commitAll(msg string) string {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "-m", msg)
	return r.git("rev-parse", "HEAD")
}

// snapshotAt は「プロジェクトルートが root（git 根からの相対 prefix 込み）」の
// snapshot を組む。decision は印だけを持てばよい（規則は Applied しか見ない）。
func (r *correctionRepo) snapshotAt(prefix string, decisions ...model.Decision) store.Snapshot {
	return store.Snapshot{Root: filepath.Join(r.dir, filepath.FromSlash(prefix)), Decisions: decisions}
}

func correctionMark(commit string) model.AppliedMark {
	return model.AppliedMark{Kind: model.AppliedCorrection, At: "2026-08-18T00:00:00Z", Commit: commit}
}

// 是正の印が付いた commit が記録も変更していたら1件出す。
func TestCorrectionChangesRecords_FiresWhenCommitTouchesStore(t *testing.T) {
	r := newCorrectionRepo(t)
	r.write(".scholia/tags/subject.x.json", `{"id":"subject.x","name":"x","kind":"subject"}`+"\n")
	r.write("main.go", "package main\n")
	r.commitAll("seed")

	// 実装だけを直した commit（これは是正として正しい）。
	r.write("main.go", "package main // fixed\n")
	implOnly := r.commitAll("fix impl")

	// 実装と一緒に記録も変えた commit（是正ではなく精緻化）。
	r.write("main.go", "package main // fixed again\n")
	r.write(".scholia/tags/subject.x.json", `{"id":"subject.x","name":"x2","kind":"subject"}`+"\n")
	mixed := r.commitAll("fix impl and spec")

	snap := r.snapshotAt(".", model.Decision{
		ID:      "01D1",
		Applied: []model.AppliedMark{correctionMark(implOnly), correctionMark(mixed)},
	})
	got := checkCorrectionChangesRecords(snap)
	if len(got) != 1 {
		t.Fatalf("記録も変えた commit 1件だけが出るはず: %+v", got)
	}
	f := got[0]
	if f.Target != mixed {
		t.Errorf("出るべきは記録も変えた commit: got %s want %s", f.Target, mixed)
	}
	if f.Rule != RuleCorrectionChangesRecords || f.Severity != SeverityInfo || f.Tier != TierAdvisory {
		t.Errorf("規則名・重大度・層が違う: %+v", f)
	}
	if !f.AcknowledgeOnly {
		t.Error("applied[] は追記専用で印を消せない——容認でしか解消できないはず")
	}
	if f.AcknowledgedBy != "" {
		t.Errorf("容認していないのに容認済みになっている: %+v", f)
	}
}

// 是正以外の印（矛盾・却下）は、この規則の対象外である
// ——結ぶ commit を持たないので外形の検査が1つも無い（射程を実測に変える）。
func TestCorrectionChangesRecords_IgnoresNonCorrectionMarks(t *testing.T) {
	r := newCorrectionRepo(t)
	r.write(".scholia/tags/subject.x.json", `{"id":"subject.x","name":"x","kind":"subject"}`+"\n")
	r.commitAll("seed")
	r.write(".scholia/tags/subject.x.json", `{"id":"subject.x","name":"x2","kind":"subject"}`+"\n")
	recordOnly := r.commitAll("spec only")

	snap := r.snapshotAt(".", model.Decision{
		ID: "01D1",
		Applied: []model.AppliedMark{
			{Kind: model.AppliedConflict, At: "2026-08-18T00:00:00Z"},
			{Kind: model.AppliedRejection, At: "2026-08-18T00:00:00Z", Decision: "01D2"},
		},
	})
	if got := checkCorrectionChangesRecords(snap); len(got) != 0 {
		t.Fatalf("矛盾・却下の印は対象外のはず: %+v", got)
	}

	// 同じ commit に是正の印を付ければ出る——**対象外なのは commit ではなく
	// 種別である**ことを、同じ入力の対で示す。
	snap.Decisions[0].Applied = append(snap.Decisions[0].Applied, correctionMark(recordOnly))
	if got := checkCorrectionChangesRecords(snap); len(got) != 1 {
		t.Fatalf("是正の印なら出るはず（対象外なのは種別）: %+v", got)
	}
}

// 対象レコード宛て acknowledges で畳める（decision-stale と同型）。
func TestCorrectionChangesRecords_FoldsWithAcknowledges(t *testing.T) {
	r := newCorrectionRepo(t)
	r.write(".scholia/tags/subject.x.json", `{"id":"subject.x","name":"x","kind":"subject"}`+"\n")
	r.write("main.go", "package main\n")
	r.commitAll("seed")
	r.write("main.go", "package main // fixed\n")
	r.write(".scholia/tags/subject.x.json", `{"id":"subject.x","name":"x2","kind":"subject"}`+"\n")
	mixed := r.commitAll("mixed")

	snap := r.snapshotAt(".", model.Decision{
		ID:           "01D1",
		Acknowledges: []string{RuleCorrectionChangesRecords},
		Applied:      []model.AppliedMark{correctionMark(mixed)},
	})
	got := checkCorrectionChangesRecords(snap)
	if len(got) != 1 {
		t.Fatalf("容認しても finding 自体は出る（畳むのは消費側）: %+v", got)
	}
	if got[0].AcknowledgedBy != "01D1" {
		t.Fatalf("容認済みとして印が付くはず: %+v", got[0])
	}
}

// 🔴 リポジトリ根より下にストアがある形（monorepo）でも効くこと。
// **固定接頭辞で判定していない**ことの実測——decision-stale はその形のバグを
// 持っており、同じ穴をここに作らないためにこの検査が要る。
func TestCorrectionChangesRecords_StoreBelowRepoRoot(t *testing.T) {
	r := newCorrectionRepo(t)
	r.write("proj/.scholia/tags/subject.x.json", `{"id":"subject.x","name":"x","kind":"subject"}`+"\n")
	r.write("proj/main.go", "package main\n")
	r.write("other/readme.md", "other\n")
	r.commitAll("seed")

	// proj のストアを触った commit。
	r.write("proj/main.go", "package main // fixed\n")
	r.write("proj/.scholia/tags/subject.x.json", `{"id":"subject.x","name":"x2","kind":"subject"}`+"\n")
	mixed := r.commitAll("mixed under proj")

	// proj の外だけを触った commit（proj のストアは動いていない）。
	r.write("other/readme.md", "other 2\n")
	elsewhere := r.commitAll("elsewhere")

	snap := r.snapshotAt("proj", model.Decision{
		ID:      "01D1",
		Applied: []model.AppliedMark{correctionMark(mixed), correctionMark(elsewhere)},
	})
	got := checkCorrectionChangesRecords(snap)
	if len(got) != 1 || got[0].Target != mixed {
		t.Fatalf("リポジトリ根より下のストアで1件だけ出るはず: %+v", got)
	}
}

// git 管理下でない・snapshot に Root が無いときは何も出さない（判定材料が無い）。
func TestCorrectionChangesRecords_SilentWithoutGit(t *testing.T) {
	mark := correctionMark("0123456789abcdef0123456789abcdef01234567")
	d := model.Decision{ID: "01D1", Applied: []model.AppliedMark{mark}}

	if got := checkCorrectionChangesRecords(store.Snapshot{Decisions: []model.Decision{d}}); got != nil {
		t.Fatalf("Root が空なら何も出さないはず: %+v", got)
	}

	// git 管理下でも、その commit がこの clone に無ければ判定できない
	// （浅い clone がこの形になる）。**在ると決めつけて finding を出さない。**
	r := newCorrectionRepo(t)
	r.write(".scholia/tags/subject.x.json", `{"id":"subject.x","name":"x","kind":"subject"}`+"\n")
	r.commitAll("seed")
	if got := checkCorrectionChangesRecords(r.snapshotAt(".", d)); len(got) != 0 {
		t.Fatalf("手元に無い commit について finding を出してはいけない: %+v", got)
	}
}

// storePathspec は「ストアが git 根の直下か、その下か」を分ける純関数
// （CLAUDE.md 1: 入力と出力の対で見る）。
func TestStorePathspec(t *testing.T) {
	cases := []struct{ prefix, want string }{
		{"", ".scholia"},
		{".", ".scholia"},
		{"proj", "proj/.scholia"},
		{"a/b", "a/b/.scholia"},
	}
	for _, c := range cases {
		if got := storePathspec(c.prefix, store.DirName); got != c.want {
			t.Errorf("storePathspec(%q) = %q, want %q", c.prefix, got, c.want)
		}
	}
}
