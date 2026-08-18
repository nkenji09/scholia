package lint

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/gitio"
	"github.com/nkenji09/scholia/internal/gittest"
	"github.com/nkenji09/scholia/internal/store"
)

// staleRepo は decision-stale を「入力（git 履歴）→ 出力（findings）」の対で
// 検査するための合成 repo。実プロジェクトの履歴を一切読まないので、この規則を
// 検査していない作業が commit を積んでも結果が動かない。
type staleRepo struct {
	t   *testing.T
	dir string
	// storeRel はリポジトリ根から見たプロジェクトルートの位置（"" なら根そのもの）。
	// 零値が従来の「根にストアがある」形なので、既存の検査は書き換えずに済む。
	storeRel string
}

func newStaleRepo(t *testing.T) *staleRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	r := &staleRepo{t: t, dir: t.TempDir()}
	gittest.InitRepo(t, r.dir)
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

// projectRoot は .scholia の親（＝Snapshot.Root に渡す値）。
func (r *staleRepo) projectRoot() string {
	if r.storeRel == "" {
		return r.dir
	}
	return filepath.Join(r.dir, filepath.FromSlash(r.storeRel))
}

func (r *staleRepo) write(relPath, body string) {
	r.t.Helper()
	p := filepath.Join(r.projectRoot(), filepath.FromSlash(relPath))
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
	for _, f := range r.findings() {
		if f.Rule != "decision-stale" {
			r.t.Fatalf("checkDecisionStale が別規則を返した: %+v", f)
		}
		out = append(out, f.Target)
	}
	return out
}

// findings は checkDecisionStale の返り値をそのまま返す（規則の種類も見たい検査用）。
func (r *staleRepo) findings() []Finding {
	r.t.Helper()
	return checkDecisionStale(store.Snapshot{Root: r.projectRoot()})
}

// writeTagFile は任意のファイル名でタグを1件書く（ファイル名そのものを動かす検査用）。
func (r *staleRepo) writeTagFile(fileName, id, name string) {
	r.t.Helper()
	r.write(".scholia/tags/"+fileName,
		`{"id":"`+id+`","name":"`+name+`","kind":"subject"}`+"\n")
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

// --- 01M0AJDYP9524AKXFN6J3FXBYJ / 01M0AJDYJSEVCSYEV0HDPSTWFZ の歯止め ---

// TestDecisionStaleRecordFileNameDoesNotChangeTheAnswer は、**記録のファイル名が
// 何であっても答えが変わらない**ことを見る。
//
// git は既定（core.quotePath=true）で、非 ASCII や引用が要る文字を含むパスを
// C 形式の引用で出す。出力のパスを前方一致で判定する形は、引用符が1つ頭に付く
// だけで必ず外れ、**検査は「異常なし」を返す**（実測: 同じ手順で ASCII 名は
// 検知され、非 ASCII 名は検知されなかった）。
//
// ⚠️ **期待値を「ASCII の対照と同じ答え」とは書かない。** 対照と比べる形は、
// 両方が同時に壊れたときに差が出ない——期待する答えそのものを書く。
//
// 落ちない範囲: ここが動かすのは**ファイル名だけ**である。窓の境界・decision の
// 同伴・ストアの位置は、それぞれ別の検査が持つ。
func TestDecisionStaleRecordFileNameDoesNotChangeTheAnswer(t *testing.T) {
	names := []struct {
		label    string
		fileName string
		id       string
	}{
		{"ASCII", "subject.ascii.json", "subject.ascii"},
		{"非 ASCII", "subject.核心.json", "subject.核心"},
		{"空白入り", "subject.with space.json", "subject.with space"},
		{"引用符とバックスラッシュ", `subject.quote"back\slash.json`, "subject.q"},
	}
	for _, n := range names {
		t.Run(n.label, func(t *testing.T) {
			if runtime.GOOS == "windows" && strings.ContainsAny(n.fileName, `"\`) {
				t.Skip(`windows のファイル名に " と \ は置けない`)
			}
			r := newStaleRepo(t)
			r.writeTagFile(n.fileName, n.id, "名前")
			r.commit("add record")

			r.writeTagFile(n.fileName, n.id, "名前v2")
			r.commit("modify record without decision")

			head := headHash(r)
			got := r.findings()
			if len(got) != 1 {
				t.Fatalf("decision 非同伴のレコード変更が1件出るはず（ファイル名 %q）: %+v", n.fileName, got)
			}
			f := got[0]
			if f.Rule != "decision-stale" || f.Target != head || f.TargetType != "commit" {
				t.Fatalf("finding の内容が想定と違う: %+v（head=%s）", f, head)
			}
			// メッセージには**変更されたレコードのファイル名**が入る。ここまで見るのは、
			// 「commit は挙がるがどのレコードか分からない」形の壊れ方を落とすため。
			if !strings.Contains(f.Message, n.fileName) {
				t.Fatalf("メッセージに変更レコード名 %q が出ていない: %s", n.fileName, f.Message)
			}
		})
	}
}

// TestDecisionStaleStoreBelowRepoRoot は、**ストアがリポジトリの根に無くても**
// 検知が外れないことを見る。
//
// 記録ディレクトリの位置を固定の接頭辞（".scholia/"）で決める形は、リポジトリ根の
// 下にストアがあると git の出す "sub/.scholia/..." と一致せず、**非 ASCII が
// 1文字も無くても最初から0件になる**（実測）。
func TestDecisionStaleStoreBelowRepoRoot(t *testing.T) {
	r := newStaleRepo(t)
	r.storeRel = "sub"

	r.writeTagFile("subject.ascii.json", "subject.ascii", "名前")
	r.commit("add record")

	r.writeTagFile("subject.ascii.json", "subject.ascii", "名前v2")
	r.commit("modify record without decision")

	head := headHash(r)
	got := r.findings()
	if len(got) != 1 || got[0].Rule != "decision-stale" || got[0].Target != head {
		t.Fatalf("リポジトリ根の下のストアでも1件出るはず: %+v（head=%s）", got, head)
	}
}

// TestDecisionStaleNamesGitDerivationFailure は、**git 管理下なのに導出が落ちた**
// とき、0件（＝異常なし）ではなく「検査していない」を1件出すことを見る。
//
// 直す前は、走査窓の commit のオブジェクトを1つ壊すと `scholia lint` が
// 「問題は見つかりませんでした」と言い exit 0 で通った（実測）。git が書いた
// fatal はどこにも出なかった。
func TestDecisionStaleNamesGitDerivationFailure(t *testing.T) {
	r := newStaleRepo(t)
	r.writeTagFile("subject.ascii.json", "subject.ascii", "名前")
	r.commit("add record")
	r.writeTagFile("subject.ascii.json", "subject.ascii", "名前v2")
	r.commit("modify record without decision")

	// 壊す前は decision-stale が1件出る（対照）。
	if got := r.findings(); len(got) != 1 || got[0].Rule != "decision-stale" {
		t.Fatalf("壊す前は decision-stale が1件出るはず: %+v", got)
	}

	// HEAD の tree オブジェクトを消す＝git 管理下ではあるが `git log` が落ちる。
	breakHeadTree(r)

	got := r.findings()
	if len(got) != 1 {
		t.Fatalf("導出が落ちたときは1件名乗るはず（0件は「異常なし」と読まれる）: %+v", got)
	}
	f := got[0]
	if f.Rule != RuleGitDerivationFailed {
		t.Fatalf("規則 id が違う: %+v", f)
	}
	if f.AcknowledgeOnly {
		t.Fatal("AcknowledgeOnly を立ててはいけない（容認で畳む対象ではないし、既定の画面で件数に畳まれる）")
	}
	if f.Severity != SeverityInfo || f.Tier != TierAdvisory {
		t.Fatalf("severity/tier が想定と違う: %+v", f)
	}
	// ⚠️ git の文言そのものは照合しない（版と locale で変わる）。見るのは
	// 「素の exit status より長い＝git が書いた理由が本文に載っている」ことである。
	if !strings.Contains(f.Message, "exit status") {
		t.Fatalf("git の終了状態が本文に載っていない: %s", f.Message)
	}
	if !strings.Contains(f.Message, "検査していません") {
		t.Fatalf("「検査していない」ことが本文に出ていない: %s", f.Message)
	}
	t.Logf("名乗った本文: %s", f.Message)
}

// TestDecisionStaleSilentWhenNotGitManaged は、**git 管理下でないときは黙る**
// ことを見る（既決の範囲・01M09FHDJCV2WWFC7Z8331B0YQ）。導出の失敗を名乗る
// 変更が、この既決まで巻き込んで名乗り始めていないかの歯止め。
func TestDecisionStaleSilentWhenNotGitManaged(t *testing.T) {
	dir := t.TempDir() // git 管理下でない
	if got := checkDecisionStale(store.Snapshot{Root: dir}); len(got) != 0 {
		t.Fatalf("git 管理下でないストアでは何も出さないはず: %+v", got)
	}
}

// TestStaleCommitsJudgesFromValues は、「どの commit を挙げるか」の判断を
// git から切り離して、入力と出力の対で検査する（CLAUDE.md 1）。
// 画面も git も起こさないので、綴りを変えても答えが変われば落ちる。
func TestStaleCommitsJudgesFromValues(t *testing.T) {
	tests := []struct {
		name        string
		storePrefix string
		commit      gitio.Commit
		want        []string // 期待するレコード basename（空なら finding にしない）
	}{
		{
			name:        "レコードの変更だけ・decision 非同伴なら挙げる",
			storePrefix: ".scholia",
			commit: gitio.Commit{Hash: "h1", Changes: []gitio.Change{
				{Status: "M", Path: ".scholia/tags/subject.核心.json"},
			}},
			want: []string{"subject.核心.json"},
		},
		{
			name:        "同じ commit に decision の追加があれば挙げない",
			storePrefix: ".scholia",
			commit: gitio.Commit{Hash: "h2", Changes: []gitio.Change{
				{Status: "M", Path: ".scholia/tags/subject.x.json"},
				{Status: "A", Path: ".scholia/decisions/01X.json"},
			}},
			want: nil,
		},
		{
			name:        "ストアがリポジトリ根の下でも同じ答えになる",
			storePrefix: "sub/.scholia",
			commit: gitio.Commit{Hash: "h3", Changes: []gitio.Change{
				{Status: "M", Path: "sub/.scholia/vocab/act.x.json"},
			}},
			want: []string{"act.x.json"},
		},
		{
			name:        "ストアの外の変更は数えない",
			storePrefix: ".scholia",
			commit: gitio.Commit{Hash: "h4", Changes: []gitio.Change{
				{Status: "M", Path: "internal/lint/rules.go"},
				{Status: "M", Path: "other/.scholia/tags/subject.x.json"},
			}},
			want: nil,
		},
		{
			name:        "rename と新規追加と削除は数えない",
			storePrefix: ".scholia",
			commit: gitio.Commit{Hash: "h5", Changes: []gitio.Change{
				{Status: "R100", Path: ".scholia/tags/subject.new.json"},
				{Status: "A", Path: ".scholia/tags/subject.added.json"},
				{Status: "D", Path: ".scholia/tags/subject.gone.json"},
			}},
			want: nil,
		},
		{
			name:        "decisions 配下の変更（M）は同伴にならない",
			storePrefix: ".scholia",
			commit: gitio.Commit{Hash: "h6", Changes: []gitio.Change{
				{Status: "M", Path: ".scholia/tags/subject.x.json"},
				{Status: "M", Path: ".scholia/decisions/01X.json"},
			}},
			want: []string{"subject.x.json"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := staleCommits([]gitio.Commit{tc.commit}, tc.storePrefix)
			if len(tc.want) == 0 {
				if len(got) != 0 {
					t.Fatalf("挙げないはずが挙がった: %+v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("1件挙がるはずが %d 件: %+v", len(got), got)
			}
			if got[0].hash != tc.commit.Hash {
				t.Fatalf("hash が違う: got %q want %q", got[0].hash, tc.commit.Hash)
			}
			if len(got[0].records) != len(tc.want) {
				t.Fatalf("レコードが違う:\n got %v\nwant %v", got[0].records, tc.want)
			}
			for i := range tc.want {
				if got[0].records[i] != tc.want[i] {
					t.Fatalf("レコードが違う:\n got %v\nwant %v", got[0].records, tc.want)
				}
			}
		})
	}
}

// breakHeadTree は HEAD の tree オブジェクトを消す。git 管理下であること
// （rev-parse --show-toplevel）は変わらないまま、`git log --name-status` だけが
// 落ちる状態を作る——package doc の3段のうち第3段そのものである。
func breakHeadTree(r *staleRepo) {
	r.t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
	cmd.Dir = r.dir
	out, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("git rev-parse HEAD^{tree}: %v", err)
	}
	obj := strings.TrimSpace(string(out))
	p := filepath.Join(r.dir, ".git", "objects", obj[:2], obj[2:])
	if err := os.Remove(p); err != nil {
		r.t.Skipf("tree オブジェクトが loose でないため壊せない（pack 済み）: %v", err)
	}
}
