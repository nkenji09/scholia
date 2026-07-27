package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `scholia review add` は .scholia/reviews/<id>.json を作り、`scholia review list` で読める（§8.4）。
func TestCLI_ReviewAddAndList(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "subject.auth", "--name", "認証", "--kind", "subject"); err != nil {
		t.Fatalf("tag create: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "action", "act.a", "--label", "a"); err != nil {
		t.Fatalf("vocab add: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "effect", "eff.a", "--label", "a"); err != nil {
		t.Fatalf("vocab add effect: %v", err)
	}
	if _, err := run(t, dir, "tx", "add", "T-1", "--action", "act.a", "--then", "eff.a", "--tags", "subject.auth"); err != nil {
		t.Fatalf("tx add: %v", err)
	}

	addOut, err := run(t, dir, "review", "add", "--on", "transition:T-1", "--body", "AI: これはテスト提案の理由", "--json")
	if err != nil {
		t.Fatalf("review add failed: %v\noutput:\n%s", err, addOut)
	}
	var added struct {
		ID        string `json:"id"`
		RecordRef struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"recordRef"`
		Body      string `json:"body"`
		Source    string `json:"source"`
		CreatedAt string `json:"createdAt"`
	}
	if err := json.Unmarshal([]byte(addOut), &added); err != nil {
		t.Fatalf("unmarshal: %v\noutput:\n%s", err, addOut)
	}
	if added.Source != "ai" {
		t.Fatalf("既定 source は ai であるべき: got %q", added.Source)
	}
	if added.RecordRef.Type != "transition" || added.RecordRef.ID != "T-1" {
		t.Fatalf("recordRef が期待通りでない: %+v", added.RecordRef)
	}

	reviewPath := filepath.Join(dir, ".scholia", "reviews", added.ID+".json")
	if _, err := os.Stat(reviewPath); err != nil {
		t.Fatalf(".scholia/reviews/%s.json が生成されていない: %v", added.ID, err)
	}

	listOut, err := run(t, dir, "review", "list", "--json")
	if err != nil {
		t.Fatalf("review list failed: %v\noutput:\n%s", err, listOut)
	}
	var listed []struct {
		ID   string `json:"id"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("unmarshal list: %v\noutput:\n%s", err, listOut)
	}
	if len(listed) != 1 || listed[0].ID != added.ID {
		t.Fatalf("list が期待通りでない: %+v", listed)
	}

	// --on フィルタで絞り込める。
	filteredOut, err := run(t, dir, "review", "list", "--on", "tag:subject.auth", "--json")
	if err != nil {
		t.Fatalf("review list --on failed: %v\noutput:\n%s", err, filteredOut)
	}
	var filtered []json.RawMessage
	if err := json.Unmarshal([]byte(filteredOut), &filtered); err != nil {
		t.Fatalf("unmarshal filtered: %v", err)
	}
	if len(filtered) != 0 {
		t.Fatalf("tag:subject.auth に一致する review は無いはず: %+v", filtered)
	}

	// scholia lint はレビューの存在に無影響で緑のまま（§8.4: reviews は store.LoadAll から不可視）。
	// info レベルの decision-coverage 指摘（T-1 に decision 未記録）は review とは無関係で
	// exit success のまま（lint.HasError は error レベルのみで fail させる）。
	lintOut, err := run(t, dir, "lint")
	if err != nil {
		t.Fatalf("lint should stay green with reviews present: %v\noutput:\n%s", err, lintOut)
	}
	if strings.Contains(lintOut, "review") {
		t.Fatalf("lint output should not reference reviews (invisible to LoadAll): %s", lintOut)
	}
}

// 存在しない対象への review add はエラーになる。
func TestCLI_ReviewAddRejectsMissingTarget(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "review", "add", "--on", "transition:does-not-exist", "--body", "x"); err == nil {
		t.Fatalf("expected error for nonexistent transition target")
	}
	if _, err := run(t, dir, "review", "add", "--on", "transition:T-1"); err == nil {
		t.Fatalf("expected error for missing --body")
	}
	if _, err := run(t, dir, "review", "add", "--body", "x"); err == nil {
		t.Fatalf("expected error for missing --on")
	}
}

// review が無いときの list は空配列（null ではない）。
func TestCLI_ReviewListEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	out, err := run(t, dir, "review", "list", "--json")
	if err != nil {
		t.Fatalf("review list failed: %v\noutput:\n%s", err, out)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("空の review list は [] であるべき: got %q", out)
	}
}

// setupReviewFixture は review adopt/reject/rm 系テスト共通の下ごしらえ
// （T-1 と review 1件）。
func setupReviewFixture(t *testing.T, dir string) (reviewID string) {
	t.Helper()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "subject.auth", "--name", "認証", "--kind", "subject"); err != nil {
		t.Fatalf("tag create: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "action", "act.a", "--label", "a"); err != nil {
		t.Fatalf("vocab add: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "effect", "eff.a", "--label", "a"); err != nil {
		t.Fatalf("vocab add effect: %v", err)
	}
	if _, err := run(t, dir, "tx", "add", "T-1", "--action", "act.a", "--then", "eff.a", "--tags", "subject.auth"); err != nil {
		t.Fatalf("tx add: %v", err)
	}
	addOut, err := run(t, dir, "review", "add", "--on", "transition:T-1", "--body", "AI: これは提案理由です", "--json")
	if err != nil {
		t.Fatalf("review add failed: %v\noutput:\n%s", err, addOut)
	}
	var added struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(addOut), &added); err != nil {
		t.Fatalf("unmarshal: %v\noutput:\n%s", err, addOut)
	}
	return added.ID
}

// tx.review.adopt: 昇格（decision 作成）→ 削除の順序で行われ、review の本文が
// decision の why に載る。
func TestCLI_ReviewAdopt(t *testing.T) {
	dir := t.TempDir()
	id := setupReviewFixture(t, dir)

	out, err := run(t, dir, "review", "adopt", id, "--json")
	if err != nil {
		t.Fatalf("review adopt failed: %v\noutput:\n%s", err, out)
	}
	var d struct {
		ID     string `json:"id"`
		Target struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"target"`
		Why string `json:"why"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("unmarshal decision: %v\noutput:\n%s", err, out)
	}
	if d.Target.Type != "transition" || d.Target.ID != "T-1" {
		t.Fatalf("decision target = %+v, want transition:T-1", d.Target)
	}
	if d.Why != "AI: これは提案理由です" {
		t.Fatalf("decision why = %q, want review body verbatim", d.Why)
	}

	if _, err := os.Stat(filepath.Join(dir, ".scholia", "decisions", d.ID+".json")); err != nil {
		t.Fatalf("decision file not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".scholia", "reviews", id+".json")); !os.IsNotExist(err) {
		t.Fatalf("review should be deleted after adopt, stat err = %v", err)
	}
}

// tx.review.reject: adopt と同じ昇格＋掃除だが why に却下である旨が前置きされる。
func TestCLI_ReviewReject(t *testing.T) {
	dir := t.TempDir()
	id := setupReviewFixture(t, dir)

	out, err := run(t, dir, "review", "reject", id, "--json")
	if err != nil {
		t.Fatalf("review reject failed: %v\noutput:\n%s", err, out)
	}
	var d struct {
		ID  string `json:"id"`
		Why string `json:"why"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("unmarshal decision: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(d.Why, "却下") || !strings.Contains(d.Why, "AI: これは提案理由です") {
		t.Fatalf("decision why = %q, want rejection prefix + review body", d.Why)
	}
	if _, err := os.Stat(filepath.Join(dir, ".scholia", "reviews", id+".json")); !os.IsNotExist(err) {
		t.Fatalf("review should be deleted after reject, stat err = %v", err)
	}
}

// --why を渡すと review 本文の代わりにそちらが decision.why になる。
func TestCLI_ReviewAdopt_WhyOverride(t *testing.T) {
	dir := t.TempDir()
	id := setupReviewFixture(t, dir)

	out, err := run(t, dir, "review", "adopt", id, "--why", "編集後の確定 why", "--json")
	if err != nil {
		t.Fatalf("review adopt failed: %v\noutput:\n%s", err, out)
	}
	var d struct {
		Why string `json:"why"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.Why != "編集後の確定 why" {
		t.Fatalf("decision why = %q, want override", d.Why)
	}
}

// tx.cli.review-rm: escape hatch — decision を残さず review だけ消える。
func TestCLI_ReviewRm(t *testing.T) {
	dir := t.TempDir()
	id := setupReviewFixture(t, dir)

	out, err := run(t, dir, "review", "rm", id)
	if err != nil {
		t.Fatalf("review rm failed: %v\noutput:\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".scholia", "reviews", id+".json")); !os.IsNotExist(err) {
		t.Fatalf("review should be deleted, stat err = %v", err)
	}
	decisionsDir := filepath.Join(dir, ".scholia", "decisions")
	entries, err := os.ReadDir(decisionsDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadDir decisions: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("review rm should not leave a decision behind, found: %+v", entries)
	}
}

// 存在しない id は adopt/reject/rm いずれもエラーになる（cond.review-exists）。
func TestCLI_ReviewAdoptRejectRm_MissingIDIsError(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "review", "adopt", "does-not-exist"); err == nil {
		t.Fatalf("expected error adopting a nonexistent review")
	}
	if _, err := run(t, dir, "review", "reject", "does-not-exist"); err == nil {
		t.Fatalf("expected error rejecting a nonexistent review")
	}
	if _, err := run(t, dir, "review", "rm", "does-not-exist"); err == nil {
		t.Fatalf("expected error removing a nonexistent review")
	}
}

// review の対象が vocab のときは decision 化できない（model.DecisionTarget
// は transition/tag のみ）。
func TestCLI_ReviewAdopt_VocabTargetIsError(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "action", "act.a", "--label", "a"); err != nil {
		t.Fatalf("vocab add: %v", err)
	}
	addOut, err := run(t, dir, "review", "add", "--on", "vocab:act.a", "--body", "AI: 語彙への提案", "--json")
	if err != nil {
		t.Fatalf("review add: %v\noutput:\n%s", err, addOut)
	}
	var added struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(addOut), &added); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, err := run(t, dir, "review", "adopt", added.ID); err == nil {
		t.Fatalf("expected error adopting a vocab-targeted review")
	}
}

// --- 現行性リンクの結線（adopt が supersedes まで束ねる・01KYHE08WNA8H1Q1DM2H45Y4TK） ---

// supersedeFixture は「旧 decision が1件ある transition」を作り、その旧 id を返す。
func supersedeFixture(t *testing.T, dir string) (oldDecisionID string) {
	t.Helper()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "subject.auth", "--name", "認証", "--kind", "subject"); err != nil {
		t.Fatalf("tag create: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "action", "act.a", "--label", "a"); err != nil {
		t.Fatalf("vocab add: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "effect", "eff.a", "--label", "a"); err != nil {
		t.Fatalf("vocab add effect: %v", err)
	}
	if _, err := run(t, dir, "tx", "add", "T-1", "--action", "act.a", "--then", "eff.a", "--tags", "subject.auth"); err != nil {
		t.Fatalf("tx add: %v", err)
	}
	out, err := run(t, dir, "decide", "--on", "transition:T-1", "--why", "旧: A とする", "--json")
	if err != nil {
		t.Fatalf("decide: %v\noutput:\n%s", err, out)
	}
	var env struct {
		Record struct {
			ID string `json:"id"`
		} `json:"record"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("unmarshal decide envelope: %v\noutput:\n%s", err, out)
	}
	if env.Record.ID == "" {
		t.Fatalf("decide が id を返していない:\n%s", out)
	}
	return env.Record.ID
}

type adoptedDecision struct {
	ID         string `json:"id"`
	Why        string `json:"why"`
	Supersedes []struct {
		ID   string `json:"id"`
		Mode string `json:"mode"`
	} `json:"supersedes"`
	Advisories []struct {
		Rule string `json:"rule"`
	} `json:"advisories"`
}

func addReviewWithSupersedes(t *testing.T, dir string, specs ...string) string {
	t.Helper()
	args := []string{"review", "add", "--on", "transition:T-1", "--body", "AI: 改訂: A ではなく B とする", "--json"}
	for _, s := range specs {
		args = append(args, "--supersedes", s)
	}
	out, err := run(t, dir, args...)
	if err != nil {
		t.Fatalf("review add --supersedes: %v\noutput:\n%s", err, out)
	}
	var added struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(out), &added); err != nil {
		t.Fatalf("unmarshal review: %v\noutput:\n%s", err, out)
	}
	return added.ID
}

// 回帰の芯: 提案が置き換えを宣言していれば、`review adopt` **だけ**で結線される
// （`scholia decision link` を手で叩かない）。ここが落ちると本件の不具合が再発し、
// rules --current が改訂済みの旧 decision を現行として出す。
func TestCLI_ReviewAdopt_LinksDeclaredSupersedes(t *testing.T) {
	dir := t.TempDir()
	oldID := supersedeFixture(t, dir)
	reviewID := addReviewWithSupersedes(t, dir, oldID+":supersede")

	out, err := run(t, dir, "review", "adopt", reviewID, "--json")
	if err != nil {
		t.Fatalf("review adopt: %v\noutput:\n%s", err, out)
	}
	var d adoptedDecision
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("unmarshal decision: %v\noutput:\n%s", err, out)
	}
	if len(d.Supersedes) != 1 || d.Supersedes[0].ID != oldID || d.Supersedes[0].Mode != "supersede" {
		t.Fatalf("adopt だけで結線されるべき: supersedes = %+v, want [{%s supersede}]", d.Supersedes, oldID)
	}

	// 結線の効果まで見る: --current が旧を畳み、新だけを現行として出す。
	rules, err := run(t, dir, "rules", "--tx", "T-1", "--current")
	if err != nil {
		t.Fatalf("rules --current: %v\noutput:\n%s", err, rules)
	}
	if strings.Contains(rules, "旧: A とする") {
		t.Fatalf("supersede した旧 decision が --current に残っている:\n%s", rules)
	}
	if !strings.Contains(rules, "改訂: A ではなく B とする") {
		t.Fatalf("新 decision が --current に出るべき:\n%s", rules)
	}
}

// mode 省略時は既定 amend（旧は失効しない）。
func TestCLI_ReviewAdopt_DefaultModeIsAmend(t *testing.T) {
	dir := t.TempDir()
	oldID := supersedeFixture(t, dir)
	reviewID := addReviewWithSupersedes(t, dir, oldID)

	out, err := run(t, dir, "review", "adopt", reviewID, "--json")
	if err != nil {
		t.Fatalf("review adopt: %v\noutput:\n%s", err, out)
	}
	var d adoptedDecision
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("unmarshal: %v\noutput:\n%s", err, out)
	}
	if len(d.Supersedes) != 1 || d.Supersedes[0].ID != oldID {
		t.Fatalf("supersedes = %+v, want 1 link to %s", d.Supersedes, oldID)
	}
	rules, err := run(t, dir, "rules", "--tx", "T-1", "--current")
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	if !strings.Contains(rules, "旧: A とする") {
		t.Fatalf("amend は旧を失効させない（--current に残るべき）:\n%s", rules)
	}
}

// 採用時にも `--supersedes` で足せる（提案時の宣言に追記される）。
func TestCLI_ReviewAdopt_FlagAddsToDeclaration(t *testing.T) {
	dir := t.TempDir()
	oldA := supersedeFixture(t, dir)
	out, err := run(t, dir, "decide", "--on", "transition:T-1", "--why", "旧B", "--json")
	if err != nil {
		t.Fatalf("decide B: %v\noutput:\n%s", err, out)
	}
	var env struct {
		Record struct {
			ID string `json:"id"`
		} `json:"record"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	oldB := env.Record.ID

	reviewID := addReviewWithSupersedes(t, dir, oldA+":amend")
	adoptOut, err := run(t, dir, "review", "adopt", reviewID, "--supersedes", oldB+":supersede", "--json")
	if err != nil {
		t.Fatalf("review adopt --supersedes: %v\noutput:\n%s", err, adoptOut)
	}
	var d adoptedDecision
	if err := json.Unmarshal([]byte(adoptOut), &d); err != nil {
		t.Fatalf("unmarshal: %v\noutput:\n%s", err, adoptOut)
	}
	if len(d.Supersedes) != 2 {
		t.Fatalf("宣言＋フラグで 2 件になるべき: %+v", d.Supersedes)
	}
	got := map[string]string{}
	for _, l := range d.Supersedes {
		got[l.ID] = l.Mode
	}
	if got[oldA] != "amend" || got[oldB] != "supersede" {
		t.Fatalf("supersedes = %+v, want %s:amend + %s:supersede", d.Supersedes, oldA, oldB)
	}
}

// 提案時の宣言を採用時に黙って書き換えない（同一 id で mode 違いは error）。
func TestCLI_ReviewAdopt_FlagCannotRewriteDeclaredMode(t *testing.T) {
	dir := t.TempDir()
	oldID := supersedeFixture(t, dir)
	reviewID := addReviewWithSupersedes(t, dir, oldID+":amend")

	if out, err := run(t, dir, "review", "adopt", reviewID, "--supersedes", oldID+":supersede"); err == nil {
		t.Fatalf("宣言済み link の mode 改変は拒否されるべき:\n%s", out)
	}
	// 失敗しても review は残る（昇格していないので掃除もしない）。
	if _, err := os.Stat(filepath.Join(dir, ".scholia", "reviews", reviewID+".json")); err != nil {
		t.Fatalf("検証で落ちたときは review を消さない: %v", err)
	}
}

// 実在しない旧 decision は宣言の時点（review add）で弾く。
func TestCLI_ReviewAdd_RejectsNonexistentSupersedeTarget(t *testing.T) {
	dir := t.TempDir()
	supersedeFixture(t, dir)
	if out, err := run(t, dir, "review", "add", "--on", "transition:T-1", "--body", "x",
		"--supersedes", "01NONEXISTENT0000000000000"); err == nil {
		t.Fatalf("実在しない旧 decision への宣言は弾かれるべき:\n%s", out)
	}
}

// 同一 id の重複指定は弾く（decide/decision link と同じ検証経路）。
func TestCLI_ReviewAdd_RejectsDuplicateSupersedeTarget(t *testing.T) {
	dir := t.TempDir()
	oldID := supersedeFixture(t, dir)
	if out, err := run(t, dir, "review", "add", "--on", "transition:T-1", "--body", "x",
		"--supersedes", oldID+":amend", "--supersedes", oldID+":supersede"); err == nil {
		t.Fatalf("重複指定は弾かれるべき:\n%s", out)
	}
}

// 不正な mode は弾く（3値のみ）。
func TestCLI_ReviewAdd_RejectsInvalidSupersedeMode(t *testing.T) {
	dir := t.TempDir()
	oldID := supersedeFixture(t, dir)
	if out, err := run(t, dir, "review", "add", "--on", "transition:T-1", "--body", "x",
		"--supersedes", oldID+":replace"); err == nil {
		t.Fatalf("mode=replace は弾かれるべき:\n%s", out)
	}
}

// reject は旧 decision を改訂も失効もさせない——宣言があっても結線しない。
func TestCLI_ReviewReject_DoesNotLinkSupersedes(t *testing.T) {
	dir := t.TempDir()
	oldID := supersedeFixture(t, dir)
	reviewID := addReviewWithSupersedes(t, dir, oldID+":supersede")

	out, err := run(t, dir, "review", "reject", reviewID, "--json")
	if err != nil {
		t.Fatalf("review reject: %v\noutput:\n%s", err, out)
	}
	var d adoptedDecision
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("unmarshal: %v\noutput:\n%s", err, out)
	}
	if len(d.Supersedes) != 0 {
		t.Fatalf("reject 経路に supersedes は載らないべき: %+v", d.Supersedes)
	}
	// reject には --supersedes フラグ自体を生やさない。
	if out, err := run(t, dir, "review", "reject", "--supersedes", oldID); err == nil || !strings.Contains(out+err.Error(), "unknown flag") {
		t.Fatalf("reject に --supersedes は無いべき: out=%s err=%v", out, err)
	}
}

// 未宣言はブロックしない。ただし対象に既存 decision があるなら advisory を添える。
func TestCLI_ReviewAdopt_UnlinkedAdvisory(t *testing.T) {
	dir := t.TempDir()
	supersedeFixture(t, dir) // T-1 に旧 decision が1件ある状態
	reviewID := addReviewWithSupersedes(t, dir)

	out, err := run(t, dir, "review", "adopt", reviewID, "--json")
	if err != nil {
		t.Fatalf("未宣言の adopt はブロックしないべき: %v\noutput:\n%s", err, out)
	}
	var d adoptedDecision
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("unmarshal: %v\noutput:\n%s", err, out)
	}
	var found bool
	for _, a := range d.Advisories {
		if a.Rule == "supersede-unlinked" {
			found = true
		}
	}
	if !found {
		t.Fatalf("supersede-unlinked advisory が出るべき: %+v\noutput:\n%s", d.Advisories, out)
	}
}

// 対象に既存 decision が無ければ advisory は出ない（純粋な新規追加は正当）。
func TestCLI_ReviewAdopt_NoAdvisoryWithoutPriorDecision(t *testing.T) {
	dir := t.TempDir()
	reviewID := setupReviewFixture(t, dir) // decide していない＝T-1 に decision 0 件

	out, err := run(t, dir, "review", "adopt", reviewID, "--json")
	if err != nil {
		t.Fatalf("review adopt: %v\noutput:\n%s", err, out)
	}
	var d adoptedDecision
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("unmarshal: %v\noutput:\n%s", err, out)
	}
	for _, a := range d.Advisories {
		if a.Rule == "supersede-unlinked" {
			t.Fatalf("既存 decision が無いのに advisory が出た:\n%s", out)
		}
	}
}
