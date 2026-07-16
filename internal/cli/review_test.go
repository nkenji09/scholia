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

// T-review-adopt: 昇格（decision 作成）→ 削除の順序で行われ、review の本文が
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

// T-review-reject: adopt と同じ昇格＋掃除だが why に却下である旨が前置きされる。
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

// T-cli-review-rm: escape hatch — decision を残さず review だけ消える。
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
