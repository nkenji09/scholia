package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// setupAuthFixture creates a store with a vocab-tagged action, a nested tag
// pair, and one transition — the shared fixture for decide/rules tests.
func setupAuthFixture(t *testing.T, dir string) {
	t.Helper()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "subject.auth", "--name", "認証", "--kind", "subject")
	mustRun(t, dir, "tag", "create", "req.auth", "--name", "要件-auth", "--kind", "requirement", "--parent", "subject.auth")
	mustRun(t, dir, "vocab", "add", "action", "act.user.submit-login", "--label", "ログイン送信")
	mustRun(t, dir, "vocab", "add", "effect", "eff.session.issue-token", "--label", "トークン発行")
	mustRun(t, dir, "vocab", "tag", "act.user.submit-login", "--add", "subject.auth")
	mustRun(t, dir, "tx", "add", "T-login",
		"--action", "act.user.submit-login",
		"--then", "eff.session.issue-token",
		"--tags", "req.auth",
	)
}

func mustRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := run(t, dir, args...)
	if err != nil {
		t.Fatalf("run %v failed: %v\noutput:\n%s", args, err, out)
	}
	return out
}

// 既定は「その記録自身への decision の本文だけ」を返す
// （01KZ06SYP12ZFDG1WPNYM529D8 結論1・2）。実効タグ経由で届くものは
// 存在・経由タグ・引き方だけになり、本文は --all で読める。
func TestCLI_RulesTxGivesOwnBodyAndFoldsInherited(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)

	// T-login の実効タグは {req.auth, subject.auth}（vocab 経路＋祖先展開）。
	txDecide := mustRun(t, dir, "decide", "--on", "transition:T-login",
		"--why", "# 遷移自身への判断\n\nhttpOnly cookie でトークン発行", "--json")
	if !strings.Contains(txDecide, `"transition"`) {
		t.Fatalf("expected transition target in decide output, got %s", txDecide)
	}
	tagDecide := mustRun(t, dir, "decide", "--on", "tag:subject.auth",
		"--why", "# タグへの判断\n\nnull と空文字は同一の未入力として扱う", "--json")
	if !strings.Contains(tagDecide, `"subject.auth"`) {
		t.Fatalf("expected tag target in decide output, got %s", tagDecide)
	}

	out := mustRun(t, dir, "rules", "--tx", "T-login")
	if !strings.Contains(out, "httpOnly cookie") {
		t.Fatalf("自身への decision は本文で出るべき:\n%s", out)
	}
	if strings.Contains(out, "null と空文字") {
		t.Fatalf("経由で届く decision の本文を既定で出してはならない:\n%s", out)
	}
	// 所在は落とさない——著者が書いた見出しと経由タグが出る。
	if !strings.Contains(out, "# タグへの判断") {
		t.Fatalf("畳んだ側に著者の見出しが出るべき:\n%s", out)
	}
	if !strings.Contains(out, "subject.auth") {
		t.Fatalf("畳んだ側に経由タグが出るべき:\n%s", out)
	}

	all := mustRun(t, dir, "rules", "--tx", "T-login", "--all")
	if !strings.Contains(all, "null と空文字") || !strings.Contains(all, "httpOnly cookie") {
		t.Fatalf("--all は畳んでいるものを全部開くべき:\n%s", all)
	}
}

// 自身への decision が 0 件でも「該当なし」と書かない
// （01KZ06SYP12ZFDG1WPNYM529D8 変更4 ⚠️）。「無い」と「別の場所に在る」は
// 読み手にとって別の事実である。
func TestCLI_RulesSaysZeroOwnNotNoneWhenInheritedExists(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "tag:subject.auth", "--why", "# 共通規則\n\n認証まわりの共通規則")

	out := mustRun(t, dir, "rules", "--tx", "T-login")
	if strings.Contains(out, "該当する decision はありません") {
		t.Fatalf("経由で支配されているのに「該当なし」と書いている:\n%s", out)
	}
	if !strings.Contains(out, "0 件") {
		t.Fatalf("自身への decision が 0 件であることを述べるべき:\n%s", out)
	}
	if !strings.Contains(out, "# 共通規則") {
		t.Fatalf("経由で支配している規則の所在を出すべき:\n%s", out)
	}
	// 引き方（変更5）。畳んだ全件を開けるコマンドが出る。
	if !strings.Contains(out, "rules --tx T-login --all") {
		t.Fatalf("畳んだ側を開く引き方を出すべき:\n%s", out)
	}
	// 何も無い対象は「該当なし」のままでよい（0 件と別の事実だから）。
	mustRun(t, dir, "tag", "create", "subject.empty", "--name", "空", "--kind", "subject")
	empty := mustRun(t, dir, "rules", "--tag", "subject.empty")
	if !strings.Contains(empty, "該当する decision はありません") {
		t.Fatalf("自身にも経由にも 1 件も無いなら該当なし:\n%s", empty)
	}
}

func TestCLI_RulesTagFoldsAncestorDecisions(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "tag:subject.auth", "--why", "# 祖先タグの規則\n\n認証まわりの共通規則")
	mustRun(t, dir, "decide", "--on", "tag:req.auth", "--why", "# このタグ自身の規則\n\n要件側の判断")

	out := mustRun(t, dir, "rules", "--tag", "req.auth")
	if !strings.Contains(out, "要件側の判断") {
		t.Fatalf("自身への decision は本文で出るべき:\n%s", out)
	}
	if strings.Contains(out, "認証まわりの共通規則") {
		t.Fatalf("祖先タグ経由の本文を既定で出してはならない:\n%s", out)
	}
	if !strings.Contains(out, "祖先タグ") {
		t.Fatalf("経由の種別（祖先タグ／直接持つタグ）を区別して出すべき:\n%s", out)
	}
	if !strings.Contains(mustRun(t, dir, "rules", "--tag", "req.auth", "--all"), "認証まわりの共通規則") {
		t.Fatalf("--all で祖先分も本文で返るべき")
	}
}

// 畳んだ側は 1 件も落とさない——件数と経由タグが、--all で本文が返る集合と
// 一致する（受け入れ基準）。
func TestCLI_RulesFoldedSetMatchesAll(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "tag:subject.auth", "--why", "# 祖先1\n\n本文1")
	mustRun(t, dir, "decide", "--on", "tag:req.auth", "--why", "# 直接1\n\n本文2")
	mustRun(t, dir, "decide", "--on", "transition:T-login", "--why", "# 自身1\n\n本文3")

	def := decodeRulesJSON(t, mustRun(t, dir, "rules", "--tx", "T-login", "--json"))
	all := decodeRulesJSON(t, mustRun(t, dir, "rules", "--tx", "T-login", "--all", "--json"))

	if got, want := len(def.Decisions)+len(def.Inherited)+len(def.Withdrawn), len(all.Decisions); got != want {
		t.Fatalf("既定の 3 群の合計 %d 件が --all の %d 件と違う（畳んだ側で落ちている）", got, want)
	}
	if len(def.Inherited) == 0 {
		t.Fatalf("この fixture では経由分があるはず: %+v", def)
	}
	for _, e := range def.Inherited {
		if e.Provenance != "effective-tag" && e.Provenance != "parent" {
			t.Fatalf("畳んだ側の出自が own になっている: %+v", e)
		}
		if e.ViaTag == "" {
			t.Fatalf("畳んだ側に経由タグが載っていない: %+v", e)
		}
		if e.ID == "" {
			t.Fatalf("畳んだ側に id が載っていない: %+v", e)
		}
	}
}

func TestCLI_RulesFacetSelector(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "tag:req.auth", "--why", "# テスト用の見出し\n\n要件 facet の規則")

	out := mustRun(t, dir, "rules", "--facet", "requirement")
	if !strings.Contains(out, "要件 facet の規則") {
		t.Fatalf("rules --facet requirement should surface decisions on requirement-kind tags:\n%s", out)
	}
}

func TestCLI_RulesRejectsMultipleSelectors(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	if _, err := run(t, dir, "rules", "--tag", "subject.auth", "--tx", "T-login"); err == nil {
		t.Fatalf("expected error when --tag and --tx are both given")
	}
	if _, err := run(t, dir, "rules", "--vocab", "act.user.submit-login", "--tag", "subject.auth"); err == nil {
		t.Fatalf("expected error when --vocab and --tag are both given")
	}
}

func TestCLI_RulesVocabSelector(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	// act.user.submit-login は subject.auth タグを直接持つ（vocab tag 経路）。
	// own の vocab-target decision と、vocab.tags 経由の tag decision の両方が出る。
	mustRun(t, dir, "decide", "--on", "vocab:act.user.submit-login", "--why", "# テスト用の見出し\n\nこの語彙固有の規則")
	mustRun(t, dir, "decide", "--on", "tag:subject.auth", "--why", "# テスト用の見出し\n\nsubject.auth の共通規則")

	out := mustRun(t, dir, "rules", "--vocab", "act.user.submit-login")
	if !strings.Contains(out, "この語彙固有の規則") {
		t.Fatalf("自身への vocab-target decision は本文で出るべき:\n%s", out)
	}
	if strings.Contains(out, "subject.auth の共通規則") {
		t.Fatalf("vocab.tags 経由の本文を既定で出してはならない:\n%s", out)
	}
	if !strings.Contains(out, "直接持つタグ") {
		t.Fatalf("vocab が直接持つタグ経由であることを出すべき:\n%s", out)
	}
	if !strings.Contains(mustRun(t, dir, "rules", "--vocab", "act.user.submit-login", "--all"), "subject.auth の共通規則") {
		t.Fatalf("--all でタグ経由分も本文で返るべき")
	}
}

// --facet には「経由」という概念が無いので畳まない
// （01KZ06SYP12ZFDG1WPNYM529D8 変更1 ⚠️）。
func TestCLI_RulesFacetDoesNotFold(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "tag:req.auth", "--why", "# facet の規則\n\n要件 facet の規則の本文")

	out := decodeRulesJSON(t, mustRun(t, dir, "rules", "--facet", "requirement", "--json"))
	if len(out.Inherited) != 0 {
		t.Fatalf("--facet では畳まない: %+v", out.Inherited)
	}
	if len(out.Decisions) != 1 {
		t.Fatalf("--facet は該当タグへの decision を本文で返す: %+v", out.Decisions)
	}
}

type rulesJSON struct {
	Decisions []struct {
		ID string `json:"id"`
	} `json:"decisions"`
	Inherited []struct {
		ID         string `json:"id"`
		Provenance string `json:"provenance"`
		ViaTag     string `json:"viaTag"`
		Heading    string `json:"heading"`
		Why        string `json:"why"`
		Changed    string `json:"changed"`
	} `json:"inherited"`
	Withdrawn []struct {
		ID string `json:"id"`
	} `json:"withdrawn"`
}

func decodeRulesJSON(t *testing.T, out string) rulesJSON {
	t.Helper()
	var v rulesJSON
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("unmarshal rules --json: %v\n%s", err, out)
	}
	return v
}

// --json の畳んだ側は本文を持たない（変更6）。人が読む出力で本文を出さないのに
// 機械可読出力では出す、という食い違いを作らない。
func TestCLI_RulesJSONInheritedCarriesNoBody(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "tag:subject.auth",
		"--why", "# 経由で届く規則\n\nこの本文は畳んだ側に出てはいけない", "--changed", "何かを変えた")

	got := decodeRulesJSON(t, mustRun(t, dir, "rules", "--tx", "T-login", "--json"))
	if len(got.Inherited) != 1 {
		t.Fatalf("経由分が 1 件のはず: %+v", got.Inherited)
	}
	e := got.Inherited[0]
	if e.Why != "" || e.Changed != "" {
		t.Fatalf("畳んだ側に本文が載っている: %+v", e)
	}
	if e.Heading != "経由で届く規則" {
		t.Fatalf("著者の見出しは載せる（本文からの切り出しではない）: %+v", e)
	}
}

func TestCLI_DecideRejectsMissingTarget(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	if _, err := run(t, dir, "decide", "--on", "transition:T-missing", "--why", "# テスト用の見出し\n\nx"); err == nil {
		t.Fatalf("expected error for decide on a nonexistent transition")
	}
	if _, err := run(t, dir, "decide", "--on", "bogus:foo", "--why", "# テスト用の見出し\n\nx"); err == nil {
		t.Fatalf("expected error for an unrecognized --on target type")
	}
}
