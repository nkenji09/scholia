package activity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nkenji09/scholia/internal/gittest"
)

// ---------------------------------------------------------------------------
// 純関数（git を呼ばない・入力と出力の対で検査する・CLAUDE.md「配線ガードの
// 書き方」1）
// ---------------------------------------------------------------------------

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("time.Parse(%q): %v", s, err)
	}
	return tm
}

// TestClassify_RecordOnlyExcludedMixedCounted は decision 本文の中核判断——
// 記録だけを触った commit は数えない（循環回避）・記録と記録外を同じ commit で
// 触ったもの（mixed）は実装活動に数える——を、値の対で検査する。
func TestClassify_RecordOnlyExcludedMixedCounted(t *testing.T) {
	commits := []rawCommit{
		{hash: "a", when: mustTime(t, "2026-08-01T00:00:00Z"), paths: []string{".scholia/decisions/x.json"}},
		{hash: "b", when: mustTime(t, "2026-08-02T00:00:00Z"), paths: []string{"internal/cli/foo.go"}},
		{hash: "c", when: mustTime(t, "2026-08-02T12:00:00Z"), paths: []string{".scholia/tags/y.json", "internal/cli/z.go"}},
	}
	impl, recordOnly, activeDays := classify(commits, ".scholia", time.UTC)
	if impl != 2 {
		t.Errorf("impl = %d, want 2（b と mixed の c）", impl)
	}
	if recordOnly != 1 {
		t.Errorf("recordOnly = %d, want 1（a のみ）", recordOnly)
	}
	if activeDays != 1 {
		t.Errorf("activeDays = %d, want 1（b と c は同じ日 08-02）", activeDays)
	}
}

// TestClassify_EmptyCommitCountsNeither は変更ファイルが無い commit（--allow-empty
// 等）をどちらにも数えないことを見る。数えると「何も変えていないのに実装活動」
// という偽の活動が生まれる。
func TestClassify_EmptyCommitCountsNeither(t *testing.T) {
	commits := []rawCommit{{hash: "e", when: mustTime(t, "2026-08-01T00:00:00Z"), paths: nil}}
	impl, recordOnly, activeDays := classify(commits, ".scholia", time.UTC)
	if impl != 0 || recordOnly != 0 || activeDays != 0 {
		t.Errorf("空 commit は数えないはず: impl=%d recordOnly=%d activeDays=%d", impl, recordOnly, activeDays)
	}
}

// TestClassify_TimezoneAffectsDayBucketing は「実装が動いた日」がタイムゾーンに
// 依存することを、UTC と Asia/Tokyo で束ねる日が変わる2つの commit で検査する。
// commit1 は UTC 8/1・Tokyo 8/2、commit2 は UTC 8/2・Tokyo 8/2 になるよう選ぶ——
// UTC では2日、Tokyo では1日にまとまるはず。
func TestClassify_TimezoneAffectsDayBucketing(t *testing.T) {
	commits := []rawCommit{
		{hash: "a", when: mustTime(t, "2026-08-01T23:00:00Z"), paths: []string{"foo.go"}},
		{hash: "b", when: mustTime(t, "2026-08-02T01:00:00Z"), paths: []string{"bar.go"}},
	}
	_, _, daysUTC := classify(commits, ".scholia", time.UTC)
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	_, _, daysTokyo := classify(commits, ".scholia", tokyo)
	if daysUTC != 2 {
		t.Errorf("UTC では2日になるはず: %d", daysUTC)
	}
	if daysTokyo != 1 {
		t.Errorf("Asia/Tokyo では1日にまとまるはず（+9h で両方 8/2 10:00 前後）: %d", daysTokyo)
	}
}

// TestParseWindowCommits は windowCommits が組み立てる生の git 出力形式を、
// git を呼ばずに解釈できることを見る（exec の境界と解釈ロジックを切り離す）。
func TestParseWindowCommits(t *testing.T) {
	raw := commitHeaderMark + "hash1" + fieldSep + "2026-08-01T00:00:00+09:00\n" +
		"\n" +
		".scholia/decisions/x.json\n" +
		"internal/cli/foo.go\n" +
		commitHeaderMark + "hash2" + fieldSep + "2026-08-02T00:00:00+09:00\n" +
		"\n"
	got, err := parseWindowCommits(raw)
	if err != nil {
		t.Fatalf("parseWindowCommits: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("commit 数 = %d, want 2: %+v", len(got), got)
	}
	if got[0].hash != "hash1" || len(got[0].paths) != 2 {
		t.Errorf("1件目が不一致: %+v", got[0])
	}
	if got[1].hash != "hash2" || len(got[1].paths) != 0 {
		t.Errorf("2件目（空 commit）が不一致: %+v", got[1])
	}
}

// TestCountDecisionsInWindow は半開区間 [since, until) の境界——since ちょうど
// は入り、until ちょうどは入らない——を値の対で検査する。
func TestCountDecisionsInWindow(t *testing.T) {
	w := Window{Since: mustTime(t, "2026-08-01T00:00:00Z"), Until: mustTime(t, "2026-08-02T00:00:00Z")}
	times := []time.Time{
		mustTime(t, "2026-07-31T23:59:59Z"), // 窓の外（前）
		mustTime(t, "2026-08-01T00:00:00Z"), // since ちょうど → 入る
		mustTime(t, "2026-08-01T12:00:00Z"), // 窓の中
		mustTime(t, "2026-08-02T00:00:00Z"), // until ちょうど → 入らない
	}
	if got := countDecisionsInWindow(times, w); got != 2 {
		t.Errorf("countDecisionsInWindow = %d, want 2（since 含む・until 含まない）", got)
	}
}

func TestPathspecHelpers(t *testing.T) {
	if got := rootPathspec(""); got != "." {
		t.Errorf("rootPathspec(\"\") = %q, want \".\"", got)
	}
	if got := rootPathspec("sub/dir"); got != "sub/dir" {
		t.Errorf("rootPathspec(\"sub/dir\") = %q, want \"sub/dir\"", got)
	}
	if got := storePathspec("", ".scholia"); got != ".scholia" {
		t.Errorf("storePathspec(\"\", \".scholia\") = %q, want \".scholia\"", got)
	}
	if got := storePathspec("sub/dir", ".scholia"); got != "sub/dir/.scholia" {
		t.Errorf("storePathspec(\"sub/dir\", \".scholia\") = %q, want \"sub/dir/.scholia\"", got)
	}
}

// ---------------------------------------------------------------------------
// 合成 git repo による統合検査（decision-stale の staleRepo と同型・
// 入力（git 履歴）→出力（Report）の対で検査する）
// ---------------------------------------------------------------------------

type actRepo struct {
	t   *testing.T
	dir string
}

func newActRepo(t *testing.T) *actRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	r := &actRepo{t: t, dir: t.TempDir()}
	gittest.InitRepo(t, r.dir)
	return r
}

func (r *actRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func (r *actRepo) write(relPath, body string) {
	r.t.Helper()
	p := filepath.Join(r.dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// commitAt は決め打ちの author/committer 日時で commit する。テストが
// 「走らせた時刻」に依存しないようにするため（歯止め2 が守る性質そのもの）。
func (r *actRepo) commitAt(msg string, at time.Time) string {
	r.t.Helper()
	r.git("add", "-A")
	ts := at.Format(time.RFC3339)
	cmd := exec.Command("git", "commit", "-q", "--allow-empty", "-m", msg)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+ts, "GIT_COMMITTER_DATE="+ts)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git commit: %v\n%s", err, out)
	}
	return strings.TrimSpace(r.git("rev-parse", "HEAD"))
}

// TestCompute_WindowBoundary_HalfOpen は歯止め2（窓の端は時刻・タイムゾーンまで
// 明示する）が実際に半開区間として効くことを、境界ちょうどの commit で検査する。
// これは activity.go の windowCommits に --until を1秒引く変異を戻すと落ちる
// （実見済み・下の TestMain 注記参照は無いので、ここに明記する）。
func TestCompute_WindowBoundary_HalfOpen(t *testing.T) {
	r := newActRepo(t)
	since := mustTime(t, "2020-01-01T00:00:00+09:00")
	until := mustTime(t, "2020-01-02T00:00:00+09:00")

	r.write("a.txt", "1")
	r.commitAt("in-window just after since", since.Add(time.Second))
	r.write("a.txt", "2")
	r.commitAt("in-window just before until", until.Add(-time.Second))
	r.write("a.txt", "3")
	r.commitAt("exactly at until (excluded)", until)
	r.write("a.txt", "4")
	r.commitAt("before since (excluded)", since.Add(-time.Second))

	rep, err := Compute(Options{
		GitRoot: r.dir, StoreDirName: ".scholia",
		Window: Window{Since: since, Until: until},
		Loc:    time.UTC, Now: until,
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if rep.ImplCommits != 2 {
		t.Errorf("ImplCommits = %d, want 2（境界ちょうどの until と、窓の前の commit は除外）", rep.ImplCommits)
	}
}

// TestCompute_WindowBoundary_FractionalUntilIncludesJustBeforeSecond は
// smoke test で実見した回帰——until が端数秒を持つ（time.Now() 由来で普通に
// 起きる）とき、「1 秒引いてから RFC3339 整形（＝端数を切り捨て）」だと二重に
// ズレて until 未満のはずの commit まで落ちる。commit は秒精度なので、
// until=32.24 なら 32 ちょうどの commit は含まれるべき（32 < 32.24）。
//
// 実際に踏んだ筋書き: `scholia init` 直後に `scholia activity` を打つと、
// 両方の commit が同じ秒に収まることがあり、実装 commit が 0 件と出た
// （本来 1 件のはず）。単体検査がここまで通っていたのは、それまでの検査が
// 端数秒ゼロの until しか使っていなかったため。
func TestCompute_WindowBoundary_FractionalUntilIncludesJustBeforeSecond(t *testing.T) {
	r := newActRepo(t)
	commitSecond := mustTime(t, "2020-01-01T12:00:32+09:00")
	until := commitSecond.Add(241925000) // 2020-01-01T12:00:32.241925+09:00（端数秒あり）

	r.write("a.txt", "1")
	r.commitAt("exactly at the whole second just before the fractional until", commitSecond)

	rep, err := Compute(Options{
		GitRoot: r.dir, StoreDirName: ".scholia",
		Window: Window{Since: commitSecond.Add(-time.Hour), Until: until},
		Loc:    time.UTC, Now: until,
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if rep.ImplCommits != 1 {
		t.Errorf("ImplCommits = %d, want 1（%s の commit は until=%s より前）", rep.ImplCommits, commitSecond, until)
	}
}

// TestCompute_Shallow_WithholdsNumbers は歯止め1（浅い clone を休眠と言わない）
// を検査する。full clone では実装活動が出るのに、同じ履歴を depth 1 で
// shallow clone すると Shallow=true になり、git 由来の数を一切埋めないことを見る。
func TestCompute_Shallow_WithholdsNumbers(t *testing.T) {
	full := newActRepo(t)
	base := mustTime(t, "2026-08-01T00:00:00Z")
	for i := 0; i < 3; i++ {
		full.write("a.txt", strings.Repeat("x", i+1))
		full.commitAt("commit", base.Add(time.Duration(i)*24*time.Hour))
	}

	fullRep, err := Compute(Options{
		GitRoot: full.dir, StoreDirName: ".scholia",
		Window: Window{Since: base.Add(-time.Hour), Until: base.Add(72 * time.Hour)},
		Loc:    time.UTC, Now: base.Add(72 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Compute（full）: %v", err)
	}
	if fullRep.Shallow || fullRep.ImplCommits != 3 {
		t.Fatalf("full clone は Shallow=false・ImplCommits=3 のはず: %+v", fullRep)
	}

	shallowDir := t.TempDir()
	cmd := exec.Command("git", "clone", "--depth", "1", "--no-local", full.dir, shallowDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone --depth 1: %v\n%s", err, out)
	}

	shallowRep, err := Compute(Options{
		GitRoot: shallowDir, StoreDirName: ".scholia",
		Window: Window{Since: base.Add(-time.Hour), Until: base.Add(72 * time.Hour)},
		Loc:    time.UTC, Now: base.Add(72 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Compute（shallow）: %v", err)
	}
	if !shallowRep.Shallow {
		t.Fatalf("shallow clone は Shallow=true のはず: %+v", shallowRep)
	}
	if shallowRep.ImplCommits != 0 || shallowRep.ActiveDays != 0 || shallowRep.LastActivity != nil {
		t.Errorf("Shallow のとき git 由来の数は埋めないはず（黙って嘘の数を出さない）: %+v", shallowRep)
	}
}

// TestCompute_DecisionCountFromAt_NotFromGit は歯止め3——decision の件数は
// git ではなくレコード自身の at から数える——を検査する。同じ commit 履歴でも、
// DecisionTimes に渡す値を変えれば結果がそのまま変わることを見て、
// 「git の commit 数を見ていない」ことを実測する（見ていれば commit 数が
// どうであれ同じ数になってしまう）。
func TestCompute_DecisionCountFromAt_NotFromGit(t *testing.T) {
	r := newActRepo(t)
	r.write(".scholia/decisions/a.json", "{}")
	r.commitAt("decision commit", mustTime(t, "2026-08-01T00:00:00Z"))

	w := Window{Since: mustTime(t, "2026-08-01T00:00:00Z"), Until: mustTime(t, "2026-08-03T00:00:00Z")}
	// 窓の中に収まる at を3つ渡す。commit は1件しか無いが、3件と出るはず——
	// git の commit 数ではなく DecisionTimes を数えている証拠。
	times := []time.Time{
		mustTime(t, "2026-08-01T01:00:00Z"),
		mustTime(t, "2026-08-01T02:00:00Z"),
		mustTime(t, "2026-08-01T03:00:00Z"),
	}
	rep, err := Compute(Options{
		GitRoot: r.dir, StoreDirName: ".scholia", Window: w,
		DecisionTimes: times, Loc: time.UTC, Now: w.Until,
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if rep.DecisionsInWindow != 3 {
		t.Errorf("DecisionsInWindow = %d, want 3（git の commit 数(1)ではなく DecisionTimes(3) から数える）", rep.DecisionsInWindow)
	}
}

// TestCompute_WindowPredatesStore_Disclosed は歯止め4——窓の開始が記録
// ディレクトリの初出より前なら、その旨を開示する——を検査する。
func TestCompute_WindowPredatesStore_Disclosed(t *testing.T) {
	r := newActRepo(t)
	storeFirstSeen := mustTime(t, "2026-07-01T00:00:00Z")
	r.write(".scholia/tags/x.json", "{}")
	r.commitAt("store created", storeFirstSeen)
	r.write("a.txt", "1")
	r.commitAt("impl after store exists", storeFirstSeen.Add(24*time.Hour))

	predatesWindow := Window{Since: storeFirstSeen.Add(-48 * time.Hour), Until: storeFirstSeen.Add(48 * time.Hour)}
	rep, err := Compute(Options{
		GitRoot: r.dir, StoreDirName: ".scholia", Window: predatesWindow,
		Loc: time.UTC, Now: predatesWindow.Until,
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if !rep.WindowPredatesStore {
		t.Errorf("窓の開始がストア初出より前なので WindowPredatesStore=true のはず: %+v", rep)
	}
	if rep.StoreFirstSeen == nil || !rep.StoreFirstSeen.Equal(storeFirstSeen) {
		t.Errorf("StoreFirstSeen = %v, want %v", rep.StoreFirstSeen, storeFirstSeen)
	}

	withinWindow := Window{Since: storeFirstSeen.Add(12 * time.Hour), Until: storeFirstSeen.Add(48 * time.Hour)}
	rep2, err := Compute(Options{
		GitRoot: r.dir, StoreDirName: ".scholia", Window: withinWindow,
		Loc: time.UTC, Now: withinWindow.Until,
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if rep2.WindowPredatesStore {
		t.Errorf("窓の開始がストア初出より後なので WindowPredatesStore=false のはず: %+v", rep2)
	}
}

// TestCompute_MonorepoScoping_OnlyProjectRoot は歯止め6（粒度をストア全体に
// 留める・monorepo ではプロジェクトルート配下だけを数える）を検査する。
// リポジトリ全体には別プロジェクトの活動があるが、ストアが sub/dir/.scholia に
// あるとき、sub/dir の外の変更は実装活動にもカウントにも一切現れないはず——
// 「乗せる向きの誤り（偽の不健全）」を作らないことを見る。
func TestCompute_MonorepoScoping_OnlyProjectRoot(t *testing.T) {
	r := newActRepo(t)
	base := mustTime(t, "2026-08-01T00:00:00Z")

	r.write("sub/dir/.scholia/tags/x.json", "{}")
	r.commitAt("init store", base)

	// 別プロジェクト（プロジェクトルートの外）の活動——数えてはいけない。
	r.write("other-project/main.go", "package main")
	r.commitAt("other project work", base.Add(1*24*time.Hour))

	// プロジェクトルート配下・ストアの外——実装活動として数える。
	r.write("sub/dir/handler.go", "package handler")
	r.commitAt("in-project impl", base.Add(2*24*time.Hour))

	// プロジェクトルート配下・ストアの中だけ——記録のみ、数えない。
	r.write("sub/dir/.scholia/tags/y.json", "{}")
	r.commitAt("in-project record-only", base.Add(3*24*time.Hour))

	gitRoot, relPrefix, err := ResolveGitContext(filepath.Join(r.dir, "sub", "dir"))
	if err != nil {
		t.Fatalf("ResolveGitContext: %v", err)
	}
	if relPrefix != "sub/dir" {
		t.Fatalf("relPrefix = %q, want \"sub/dir\"", relPrefix)
	}

	rep, err := Compute(Options{
		GitRoot: gitRoot, RelPrefix: relPrefix, StoreDirName: ".scholia",
		Window: Window{Since: base.Add(-time.Hour), Until: base.Add(96 * time.Hour)},
		Loc:    time.UTC, Now: base.Add(96 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	// 数えるべき実装活動は「in-project impl」の1件だけ。他プロジェクトの1件が
	// 紛れ込んで実装活動に化けていないか（乗せる向きの誤り）をここで見る。
	// 記録のみは「init store」＋「in-project record-only」の2件
	// （他プロジェクトの commit はそもそも pathspec で見えない）。
	if rep.ImplCommits != 1 {
		t.Errorf("ImplCommits = %d, want 1（他プロジェクトの commit を数えていない）", rep.ImplCommits)
	}
	if rep.RecordOnlyCommits != 2 {
		t.Errorf("RecordOnlyCommits = %d, want 2（init store + in-project record-only）", rep.RecordOnlyCommits)
	}
}

// TestResolveGitContext_RepoRoot はストアがリポジトリの根そのものにあるとき
// RelPrefix が空になることを見る（monorepo テストの対照）。
func TestResolveGitContext_RepoRoot(t *testing.T) {
	r := newActRepo(t)
	r.write(".scholia/tags/x.json", "{}")
	r.commitAt("init", mustTime(t, "2026-08-01T00:00:00Z"))

	gitRoot, relPrefix, err := ResolveGitContext(r.dir)
	if err != nil {
		t.Fatalf("ResolveGitContext: %v", err)
	}
	if relPrefix != "" {
		t.Errorf("relPrefix = %q, want \"\"（ストアが根そのもの）", relPrefix)
	}
	if filepath.Clean(gitRoot) != filepath.Clean(evalSymlinks(t, r.dir)) {
		t.Errorf("gitRoot = %q, want %q", gitRoot, r.dir)
	}
}

func evalSymlinks(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	return resolved
}

// TestResolveGitContext_NotAGitRepo はストアが git 管理下に無いときエラーで
// 返る（呼び出し側 = internal/cli がそれを「何も出さない」開示に変える）ことを
// 見る。decision-stale と同型の「射程外を黙って通さない」検査。
func TestResolveGitContext_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := ResolveGitContext(dir); err == nil {
		t.Error("git 管理下でないディレクトリでは error を返すはず")
	}
}

// TestCompute_NonASCIIProjectRootPathsAreNotQuotedAway はクリーンルーム
// レビュー FAIL-1 の再現1（プロジェクト根が日本語の monorepo）を固定する。
//
// 既定（core.quotePath=true）だと `git log --name-only` が非 ASCII パスを
// C 形式の引用符付きで出し、`classify` の前方一致がすべて外れて**記録だけの
// commit が実装 commit に化ける**——決定本文が「循環するから」と名指しした
// 壊れ方が、パスの引用によって逆方向から再発する。
func TestCompute_NonASCIIProjectRootPathsAreNotQuotedAway(t *testing.T) {
	r := newActRepo(t)
	base := mustTime(t, "2026-08-01T00:00:00Z")

	r.write("製品A/.scholia/tags/x.json", "{}")
	r.commitAt("store init", base)
	r.write("製品A/app.go", "package main")
	r.commitAt("impl", base.Add(24*time.Hour))
	r.write("製品A/.scholia/tags/y.json", "{}")
	r.commitAt("record only", base.Add(48*time.Hour))

	gitRoot, relPrefix, err := ResolveGitContext(filepath.Join(r.dir, "製品A"))
	if err != nil {
		t.Fatalf("ResolveGitContext: %v", err)
	}

	rep, err := Compute(Options{
		GitRoot: gitRoot, RelPrefix: relPrefix, StoreDirName: ".scholia",
		Window: Window{Since: base.Add(-time.Hour), Until: base.Add(96 * time.Hour)},
		Loc:    time.UTC, Now: base.Add(96 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if rep.ImplCommits != 1 {
		t.Errorf("ImplCommits = %d, want 1（record-only の commit が引用符化で実装活動に化けていないか）", rep.ImplCommits)
	}
	if rep.RecordOnlyCommits != 2 {
		t.Errorf("RecordOnlyCommits = %d, want 2（store init + record only）", rep.RecordOnlyCommits)
	}
}

// TestCompute_NonASCIIStoreFilePathsAreNotQuotedAway はクリーンルーム
// レビュー FAIL-1 の再現2（ストア内に非 ASCII のファイル）を固定する。
// こちらはプロジェクト根が ASCII でも、ストア配下のファイル名が非 ASCII なら
// 個別に引用符化され、そのファイルだけを触った commit が実装活動に化ける。
func TestCompute_NonASCIIStoreFilePathsAreNotQuotedAway(t *testing.T) {
	r := newActRepo(t)
	base := mustTime(t, "2026-08-01T00:00:00Z")

	r.write(".scholia/tags/x.json", "{}")
	r.commitAt("store init", base)
	r.write(".scholia/notes/設計メモ.md", "memo")
	r.commitAt("non-ascii record file", base.Add(24*time.Hour))

	rep, err := Compute(Options{
		GitRoot: r.dir, StoreDirName: ".scholia",
		Window: Window{Since: base.Add(-time.Hour), Until: base.Add(48 * time.Hour)},
		Loc:    time.UTC, Now: base.Add(48 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if rep.ImplCommits != 0 {
		t.Errorf("ImplCommits = %d, want 0（非 ASCII ファイルだけを触った commit が実装活動に化けていないか）", rep.ImplCommits)
	}
	if rep.RecordOnlyCommits != 2 {
		t.Errorf("RecordOnlyCommits = %d, want 2", rep.RecordOnlyCommits)
	}
}

// TestCompute_NoCommitsYet_ReturnsZeroNotError はクリーンルームレビュー
// FAIL-2 を固定する。`git init` 直後（unborn HEAD・commit ゼロ）は異常では
// なく正当な「実装活動ゼロ」の状態——`git init` → `scholia init` →
// `scholia activity` という最も普通の初回の順番で、以前は exit status 128 に
// なっていた。
func TestCompute_NoCommitsYet_ReturnsZeroNotError(t *testing.T) {
	r := newActRepo(t)
	// commit を1件も作らない（HEAD が unborn のまま）。

	now := mustTime(t, "2026-08-18T00:00:00Z")
	rep, err := Compute(Options{
		GitRoot: r.dir, StoreDirName: ".scholia",
		Window: Window{Since: now.Add(-90 * 24 * time.Hour), Until: now},
		Loc:    time.UTC, Now: now,
	})
	if err != nil {
		t.Fatalf("Compute はエラーではなくゼロを返すはず: %v", err)
	}
	if rep.ImplCommits != 0 || rep.RecordOnlyCommits != 0 || rep.ActiveDays != 0 {
		t.Errorf("commit ゼロの repo はすべてゼロのはず: %+v", rep)
	}
	if rep.LastActivity != nil {
		t.Errorf("LastActivity は nil のはず: %v", rep.LastActivity)
	}
	if rep.Shallow {
		t.Errorf("commit ゼロの repo を Shallow=true にする理由は無い: %+v", rep)
	}
}
