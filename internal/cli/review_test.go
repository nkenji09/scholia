package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `pmem review add` は .pmem/reviews/<id>.json を作り、`pmem review list` で読める（§8.4）。
func TestCLI_ReviewAddAndList(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "subject.auth", "--name", "認証"); err != nil {
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

	reviewPath := filepath.Join(dir, ".pmem", "reviews", added.ID+".json")
	if _, err := os.Stat(reviewPath); err != nil {
		t.Fatalf(".pmem/reviews/%s.json が生成されていない: %v", added.ID, err)
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

	// pmem lint はレビューの存在に無影響で緑のまま（§8.4: reviews は store.LoadAll から不可視）。
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
