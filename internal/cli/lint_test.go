package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/lint"
)

// setupLintCoverageStore は decision-coverage の3段（via-tag / none →
// decide 後に direct）を再現する最小 store を組む。
//   - T-covered: req.a を own タグに持ち、req.a 宛 decision に via-tag で到達
//   - T-bare:    own にも実効タグにも decision なし（none）
//
// T-bare の then は eff.b（T-covered と別）にして、U2 の duplicate-atom
// advisory（同一 action＋given＋then の複製検出）に掛からないようにする。
func setupLintCoverageStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	steps := [][]string{
		{"init"},
		{"vocab", "add", "action", "act.a", "--label", "a"},
		{"vocab", "add", "effect", "eff.a", "--label", "e"},
		{"vocab", "add", "effect", "eff.b", "--label", "e2"},
		{"tag", "create", "req.a", "--name", "要件A", "--kind", "requirement"},
		{"tx", "add", "T-covered", "--action", "act.a", "--then", "eff.a", "--tags", "req.a"},
		{"tx", "add", "T-bare", "--action", "act.a", "--then", "eff.b"},
		{"decide", "--on", "tag:req.a", "--why", "# テスト用の見出し\n\nタグ側の決定"},
	}
	for _, s := range steps {
		if out, err := run(t, dir, s...); err != nil {
			t.Fatalf("%v failed: %v\noutput:\n%s", s, err, out)
		}
	}
	return dir
}

// 既定出力: decision-coverage は none のみ列挙され、3段の件数がサマリ行に出る。
func TestLintDefaultShowsOnlyNoneCoverageWithSummary(t *testing.T) {
	dir := setupLintCoverageStore(t)

	out, err := run(t, dir, "lint")
	if err != nil {
		t.Fatalf("lint failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "decision-coverage: direct 0 / via-tag 1 / none 1（via-tag の内訳は --verbose）") {
		t.Fatalf("summary line missing or wrong:\n%s", out)
	}
	if !strings.Contains(out, "transition T-bare: own にも実効タグにも decision が 1 件もありません（none・why 未記録）") {
		t.Fatalf("none finding for T-bare missing:\n%s", out)
	}
	if strings.Contains(out, "transition T-covered") {
		t.Fatalf("via-tag finding must not be listed in default output:\n%s", out)
	}

	// direct 化: T-bare に own decision を付けると none が消え direct が増える。
	if o, err := run(t, dir, "decide", "--on", "transition:T-bare", "--why", "# テスト用の見出し\n\n遷移固有の決定"); err != nil {
		t.Fatalf("decide on transition failed: %v\noutput:\n%s", err, o)
	}
	out, err = run(t, dir, "lint")
	if err != nil {
		t.Fatalf("lint failed after decide: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "decision-coverage: direct 1 / via-tag 1 / none 0") {
		t.Fatalf("summary after decide missing or wrong:\n%s", out)
	}
	if strings.Contains(out, "transition T-bare") {
		t.Fatalf("direct-covered transition must not be listed:\n%s", out)
	}
}

// --verbose: via-tag の内訳（どのタグ経由か・decision 件数）が展開される。
func TestLintVerboseExpandsViaTagProvenance(t *testing.T) {
	dir := setupLintCoverageStore(t)

	out, err := run(t, dir, "lint", "--verbose")
	if err != nil {
		t.Fatalf("lint --verbose failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "decision-coverage via-tag の内訳:") {
		t.Fatalf("verbose breakdown header missing:\n%s", out)
	}
	if !strings.Contains(out, "  T-covered: via req.a (1)") {
		t.Fatalf("verbose breakdown line for T-covered missing:\n%s", out)
	}
}

// --json: decision-coverage は direct/via-tag/none の全件が coverage（と via-tag
// の detail）付きで出る。封筒の形は不変（findings + counts）。
func TestLintJSONCarriesAllCoverageFindings(t *testing.T) {
	dir := setupLintCoverageStore(t)

	out, err := run(t, dir, "lint", "--json")
	if err != nil {
		t.Fatalf("lint --json failed: %v\noutput:\n%s", err, out)
	}
	var resp struct {
		Findings []lint.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("json decode failed: %v\noutput:\n%s", err, out)
	}
	coverage := make(map[string]lint.Finding)
	for _, f := range resp.Findings {
		if f.Coverage != "" {
			coverage[f.Target] = f
		}
	}
	if len(coverage) != 2 {
		t.Fatalf("expected coverage findings for all 2 transitions, got %+v", coverage)
	}
	if f := coverage["T-covered"]; f.Coverage != lint.CoverageViaTag || f.Detail != "via req.a (1)" {
		t.Fatalf("T-covered = %+v, want via-tag with detail 'via req.a (1)'", f)
	}
	if f := coverage["T-bare"]; f.Coverage != lint.CoverageNone {
		t.Fatalf("T-bare = %+v, want none", f)
	}
}

// --- #45 U4: lint --ci（baseline ratchet）と lint baseline update ---

// setupRatchetStore は requirement-gap warn（req.gap: 充足遷移 0 件）が 1 件
// 出る最小 store を組む。
func setupRatchetStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	steps := [][]string{
		{"init"},
		{"vocab", "add", "action", "act.a", "--label", "a"},
		{"vocab", "add", "effect", "eff.a", "--label", "e"},
		{"tag", "create", "req.gap", "--name", "未充足要件", "--kind", "requirement"},
	}
	for _, s := range steps {
		if out, err := run(t, dir, s...); err != nil {
			t.Fatalf("%v failed: %v\noutput:\n%s", s, err, out)
		}
	}
	return dir
}

func TestLintCI_NoBaselineIsInactiveRatchet(t *testing.T) {
	dir := setupRatchetStore(t)

	// baseline 不在: warn が出ていても --ci は fail しない（opt-in）。
	out, err := run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("baseline 不在の lint --ci が fail した: %v\n%s", err, out)
	}
	if !strings.Contains(out, "非活性") {
		t.Fatalf("expected inactive-ratchet note:\n%s", out)
	}
}

func TestLintCI_BaselineRatchet(t *testing.T) {
	dir := setupRatchetStore(t)

	// baseline update で現在の warn 1 件（requirement-gap req.gap）を吸収。
	out, err := run(t, dir, "lint", "baseline", "update")
	if err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}
	if !strings.Contains(out, "warn 1 件") {
		t.Fatalf("expected 1-entry baseline summary:\n%s", out)
	}

	// baseline に吸収済み → exit 0。
	out, err = run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("baseline 吸収済みの lint --ci が fail した: %v\n%s", err, out)
	}
	if !strings.Contains(out, "新規 warn 0（baseline 1 件・stale 0 件）") {
		t.Fatalf("expected ratchet summary:\n%s", out)
	}

	// 新規 warn（もう 1 つの未充足 requirement タグ）を作る → exit 1。
	if out, err := run(t, dir, "tag", "create", "req.gap2", "--name", "新規未充足", "--kind", "requirement"); err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}
	out, err = run(t, dir, "lint", "--ci")
	if err == nil {
		t.Fatalf("baseline に無い新規 warn で lint --ci が exit 0 になった:\n%s", out)
	}
	if !strings.Contains(out, "requirement-gap: req.gap2") {
		t.Fatalf("expected the new warn to be listed by rule+target:\n%s", out)
	}
	if !strings.Contains(out, "baseline update") {
		t.Fatalf("expected remediation hint:\n%s", out)
	}

	// 既定の lint は従来契約のまま（warn は exit 0）。
	if out, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("既定 lint の exit 契約が変わっている: %v\n%s", err, out)
	}
}

func TestLintCI_StaleEntryIsInfoOnly(t *testing.T) {
	dir := setupRatchetStore(t)

	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}
	// warn の原因を解消（req.gap を充足する遷移を追加）→ baseline entry が stale 化。
	if out, err := run(t, dir, "tx", "add", "T-fill", "--action", "act.a", "--then", "eff.a", "--tags", "req.gap"); err != nil {
		t.Fatalf("tx add: %v\n%s", err, out)
	}

	out, err := run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("stale entry だけの lint --ci が fail した: %v\n%s", err, out)
	}
	if !strings.Contains(out, "stale baseline entry: requirement-gap req.gap") {
		t.Fatalf("expected stale info line:\n%s", out)
	}

	// 次の baseline update で自然消滅（削除 1）。
	out, err = run(t, dir, "lint", "baseline", "update")
	if err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}
	if !strings.Contains(out, "warn 0 件") || !strings.Contains(out, "削除 1") {
		t.Fatalf("expected shrink summary:\n%s", out)
	}
}

func TestLintCI_InfoAndAdvisoryAreNotRatcheted(t *testing.T) {
	dir := setupRatchetStore(t)
	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}
	// info（unused-vocab 等）や advisory を増やしても --ci は fail しない。
	// cond.unused はどの遷移からも参照されない語彙 → unused-vocab info。
	if out, err := run(t, dir, "vocab", "add", "condition", "cond.unused", "--label", "未使用"); err != nil {
		t.Fatalf("vocab add: %v\n%s", err, out)
	}
	out, err := run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("info の増加で lint --ci が fail した（ratchet は warn 専用のはず）: %v\n%s", err, out)
	}
	if !strings.Contains(out, "新規 warn 0") {
		t.Fatalf("expected zero new warns:\n%s", out)
	}
}

func TestLintCI_JSONCarriesCIResult(t *testing.T) {
	dir := setupRatchetStore(t)
	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "tag", "create", "req.gap2", "--name", "新規未充足", "--kind", "requirement"); err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}

	out, err := run(t, dir, "lint", "--ci", "--json")
	if err == nil {
		t.Fatalf("新規 warn ありの lint --ci --json が exit 0:\n%s", out)
	}
	// エラーメッセージ行が JSON の後に混ざるため、先頭の JSON オブジェクトだけを decode する。
	dec := json.NewDecoder(strings.NewReader(out))
	var parsed struct {
		CI struct {
			BaselinePresent bool `json:"baselinePresent"`
			BaselineCount   int  `json:"baselineCount"`
			NewWarns        []struct {
				Rule   string `json:"rule"`
				Target string `json:"target"`
			} `json:"newWarns"`
		} `json:"ci"`
	}
	if err := dec.Decode(&parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !parsed.CI.BaselinePresent || parsed.CI.BaselineCount != 1 {
		t.Fatalf("ci envelope = %+v", parsed.CI)
	}
	if len(parsed.CI.NewWarns) != 1 || parsed.CI.NewWarns[0].Rule != "requirement-gap" || parsed.CI.NewWarns[0].Target != "req.gap2" {
		t.Fatalf("newWarns = %+v", parsed.CI.NewWarns)
	}
}

func TestLintBaseline_RenameRetargetsEntries(t *testing.T) {
	dir := setupRatchetStore(t)
	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}

	// tag rename → baseline 内の target id が追随し、新 id では新規 warn に
	// ならない（旧 id のままなら req.gap-renamed が新規 warn になり exit 1）。
	if out, err := run(t, dir, "tag", "rename", "req.gap", "req.gap-renamed", "--no-refs"); err != nil {
		t.Fatalf("tag rename: %v\n%s", err, out)
	}
	out, err := run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("rename 後の lint --ci が fail した（baseline 追随漏れ）: %v\n%s", err, out)
	}
	if !strings.Contains(out, "新規 warn 0（baseline 1 件・stale 0 件）") {
		t.Fatalf("expected retargeted baseline to absorb the renamed warn:\n%s", out)
	}
}

// baseline update は typed 容認（#45 D6・AcknowledgedBy）で畳んだ warn を baseline に
// 載せない——evaluateCI（lint --ci）が除外するのと揃える。揃っていないと、acknowledges で
// 解消した gap が baseline に居座り、lint --ci が「stale（次の update で消える）」と言うのに
// update を再実行しても消えない不整合が起きる（concierge が Step3 merge 後に発見したバグ）。
func TestLintBaseline_ExcludesAcknowledgedWarns(t *testing.T) {
	dir := setupRatchetStore(t) // req.gap（遷移 0）＝requirement-gap warn 1 件

	// 当該タグ宛てに acknowledges:[requirement-gap] の decision を置くと、その warn は
	// AcknowledgedBy で畳まれる（typed 容認）。
	if out, err := run(t, dir, "decide", "--on", "tag:req.gap",
		"--acknowledges", "requirement-gap", "--why", "# テスト用の見出し\n\n意図して残す gap（テスト）"); err != nil {
		t.Fatalf("decide --acknowledges: %v\n%s", err, out)
	}

	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}

	// 容認済みの warn は baseline に入らない＝baseline 0 件。バグ時は baseline 1 件で
	// stale 化し、再 update しても消えなかった。
	out, err := run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("lint --ci: %v\n%s", err, out)
	}
	if !strings.Contains(out, "baseline 0 件・stale 0 件") {
		t.Fatalf("容認済み warn が baseline に載った（AcknowledgedBy フィルタ漏れ）:\n%s", out)
	}
}

// --- --require-git-derivation（decision 01M0AJDYJSEVCSYEV0HDPSTWFZ）---

// TestCLILintRequireGitDerivation は、「git 導出が落ちた」ことの扱いを
// **既定**と**フラグを立てたとき**の対で検査する。
//
//   - 既定: 明細は出るが exit 0。記録を1バイトも変えていない利用者の CI を、
//     実行環境の変化だけで赤くしない（01KXS68HCNQ0H9QKNYFQ869J19 の理由と同型）。
//   - `--require-git-derivation`: 1件でもあれば exit 1。
//
// 落ちない範囲: ここが見るのは exit code と本文の有無だけである。3段のどれで
// 落ちたか（git が無い／管理下でない／導出が落ちた）は internal/lint の
// TestDecisionStaleNamesGitDerivationFailure と
// TestDecisionStaleSilentWhenNotGitManaged が持つ。
func TestCLILintRequireGitDerivation(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "tag", "create", "subject.x", "--name", "主題", "--kind", "subject"); err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}
	gitCommitAllT(t, dir, "seed store")

	// (1) 導出が生きているうちは、フラグを立てても緑（フラグ自体が偽陽性を出さない）。
	if out, err := run(t, dir, "lint", "--require-git-derivation"); err != nil {
		t.Fatalf("導出できているのに落ちた: %v\n%s", err, out)
	}

	// (2) git 管理下のまま `git log` だけを落とす。
	breakHeadTreeObject(t, dir)

	// 既定は exit 0 のまま。ただし**黙らない**——本文が出る。
	out, err := run(t, dir, "lint")
	if err != nil {
		t.Fatalf("既定では exit 0 のままにする: %v\n%s", err, out)
	}
	if !strings.Contains(out, lint.RuleGitDerivationFailed) {
		t.Fatalf("既定の画面に「検査していない」ことが出ていない:\n%s", out)
	}
	// 🔴 **理由まで届いているかを見る。** 「終了状態の語が在るか」だけを見る形は、
	// 標準エラーの埋め込みを外す変異を素通りさせる（クリーンルームレビュー M-O）。
	if i := strings.Index(out, "exit status"); i < 0 || len(out[i:]) <= len("exit status 128") {
		t.Fatalf("git が書いた理由が画面に届いていない（終了状態だけ）:\n%s", out)
	}
	if strings.Contains(out, "問題は見つかりませんでした") {
		t.Fatalf("検査できていないのに「問題は見つかりませんでした」と出ている:\n%s", out)
	}

	// --ci でも既定は赤くしない（ratchet には載せない）。
	if out, err := run(t, dir, "lint", "--ci"); err != nil {
		t.Fatalf("--ci 単体では赤くしない: %v\n%s", err, out)
	}

	// (3) フラグを立てたときだけ exit 1。
	if out, err := run(t, dir, "lint", "--require-git-derivation"); err == nil {
		t.Fatalf("--require-git-derivation を立てたら exit 1 にするはず:\n%s", out)
	}
	if out, err := run(t, dir, "lint", "--ci", "--require-git-derivation"); err == nil {
		t.Fatalf("--ci と併用しても exit 1 にするはず:\n%s", out)
	}
}

// TestCLIRetrofitSeparatesUnavailableFromFixable は、「検査が走らなかった」申告が
// **是正候補に混ざらない**ことを見る。混ざると、retrofit が「記録を直せば消える」
// と読める棚卸しを出す——実際には記録を直しても消えない。
func TestCLIRetrofitSeparatesUnavailableFromFixable(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "tag", "create", "subject.x", "--name", "主題", "--kind", "subject"); err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}
	gitCommitAllT(t, dir, "seed store")
	breakHeadTreeObject(t, dir)

	out, err := run(t, dir, "retrofit", "--json")
	if err != nil {
		t.Fatalf("retrofit --json: %v\n%s", err, out)
	}
	var payload struct {
		Fixable struct {
			FindingCount int `json:"findingCount"`
		} `json:"fixable"`
		AcknowledgeOnly struct {
			FindingCount int `json:"findingCount"`
		} `json:"acknowledgeOnly"`
		Unavailable []struct {
			Rule string `json:"rule"`
		} `json:"unavailable"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if len(payload.Unavailable) != 1 || payload.Unavailable[0].Rule != lint.RuleGitDerivationFailed {
		t.Fatalf("走らなかった検査が別掲されていない: %+v\n%s", payload.Unavailable, out)
	}
	if payload.Fixable.FindingCount != 0 {
		t.Fatalf("是正候補に混ざった（記録を直しても消えないものを fixable に数えている）: %d\n%s",
			payload.Fixable.FindingCount, out)
	}
	if payload.AcknowledgeOnly.FindingCount != 0 {
		t.Fatalf("acknowledge-only に混ざった（acknowledges で畳めないのに畳める区分に入っている）: %d\n%s",
			payload.AcknowledgeOnly.FindingCount, out)
	}
}

// breakHeadTreeObject は HEAD の tree オブジェクトを消す。git 管理下であることは
// 変わらないまま、`git log --name-status` だけが落ちる状態を作る。
func breakHeadTreeObject(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD^{tree}: %v", err)
	}
	obj := strings.TrimSpace(string(out))
	p := filepath.Join(dir, ".git", "objects", obj[:2], obj[2:])
	// 🔴 削れなかったら黙って skip しない（internal/lint の breakHeadTree と同じ理由）。
	if err := os.Remove(p); err != nil {
		t.Fatalf("tree オブジェクトを壊せなかった（pack 済みなら別の壊し方に変えること。"+
			"このまま skip すると、この歯止めは黙って緑になる）: %v", err)
	}
}

// TestCLILintSilentOnRepoWithNoCommits は、**commit が1件も無いリポジトリ**での
// 初回体験を、`--require-git-derivation` を立てた場合まで含めて見る。
//
// README のクイックスタートも初期設定スキルも、**記録を作ってから `git commit` を
// 1度も挟まずに `scholia lint` を打たせる**形になっている。ここで新しく警告が出ると、
// 初めて使う人が最初に見る画面に出る（差し戻し1回目で実際にそうなっていた・
// 01M0APXCFF70MBZCQT98MNQMW8）。
func TestCLILintSilentOnRepoWithNoCommits(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir) // git init だけ。commit はまだ1件も無い
	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "tag", "create", "subject.x", "--name", "主題", "--kind", "subject"); err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}

	out, err := run(t, dir, "lint")
	if err != nil {
		t.Fatalf("commit ゼロで exit 1 にしてはいけない: %v\n%s", err, out)
	}
	if strings.Contains(out, lint.RuleGitDerivationFailed) {
		t.Fatalf("commit が1件も無いだけで「検査していません」と名乗ってはいけない:\n%s", out)
	}
	if !strings.Contains(out, "問題は見つかりませんでした") {
		t.Fatalf("走査する対象がゼロなら「問題なし」が正しい答えである:\n%s", out)
	}

	// フラグを立てた利用者も、commit ゼロで落ちてはいけない。
	if out, err := run(t, dir, "lint", "--require-git-derivation"); err != nil {
		t.Fatalf("--require-git-derivation でも commit ゼロで落ちてはいけない: %v\n%s", err, out)
	}

	// retrofit の面にも出ない（別掲の行が先頭に出ていた）。
	out, err = run(t, dir, "retrofit")
	if err != nil {
		t.Fatalf("retrofit: %v\n%s", err, out)
	}
	if strings.Contains(out, lint.RuleGitDerivationFailed) {
		t.Fatalf("retrofit の面にも出してはいけない:\n%s", out)
	}
}
