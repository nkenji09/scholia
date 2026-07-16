package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

func TestInitIsIdempotentAndWritesDefaultConfig(t *testing.T) {
	dir := t.TempDir()

	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.SchemaVersion != 1 || cfg.Viewer.Port != 4577 {
		t.Fatalf("unexpected default config: %+v", cfg)
	}

	// 既存 config を書き換えて再 Init しても上書きされないこと（冪等）。
	cfg.SchemaVersion = 999
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if _, err := Init(dir); err != nil {
		t.Fatalf("second Init: %v", err)
	}
	reloaded, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig after second Init: %v", err)
	}
	if reloaded.SchemaVersion != 999 {
		t.Fatalf("Init overwrote existing config.json: got schemaVersion=%d", reloaded.SchemaVersion)
	}

	for _, sub := range []string{"vocab", "tags", "transitions", "decisions"} {
		if info, err := os.Stat(filepath.Join(dir, ".scholia", sub)); err != nil || !info.IsDir() {
			t.Fatalf(".scholia/%s missing: %v", sub, err)
		}
	}

	// .gitignore に .scholia/index.db が 1 回だけ追記される。
	if _, err := Init(dir); err != nil {
		t.Fatalf("third Init: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	count := strings.Count(string(data), ".scholia/index.db")
	if count != 1 {
		t.Fatalf(".gitignore entry duplicated or missing: count=%d, content=%q", count, string(data))
	}
}

func TestInitWritesReviewsGitignoreEntryOnce(t *testing.T) {
	dir := t.TempDir()

	if _, err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// 2 回目以降の Init でも .scholia/reviews/ が重複追記されないこと（冪等）。
	if _, err := Init(dir); err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if _, err := Init(dir); err != nil {
		t.Fatalf("third Init: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	count := strings.Count(string(data), ".scholia/reviews/")
	if count != 1 {
		t.Fatalf(".gitignore entry duplicated or missing: count=%d, content=%q", count, string(data))
	}
}

func TestInitWithOptionsSkipGitignoreLeavesGitignoreUntouched(t *testing.T) {
	dir := t.TempDir()

	if _, err := InitWithOptions(dir, InitOptions{SkipGitignore: true}); err != nil {
		t.Fatalf("InitWithOptions: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf(".gitignore should not be created when SkipGitignore=true, stat err=%v", err)
	}
	for _, sub := range []string{"vocab", "tags", "transitions", "decisions"} {
		if info, err := os.Stat(filepath.Join(dir, ".scholia", sub)); err != nil || !info.IsDir() {
			t.Fatalf(".scholia/%s missing: %v", sub, err)
		}
	}
}

func TestInitWithOptionsDefaultMatchesInit(t *testing.T) {
	dir := t.TempDir()

	if _, err := InitWithOptions(dir, InitOptions{}); err != nil {
		t.Fatalf("InitWithOptions: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), ".scholia/index.db") {
		t.Fatalf(".gitignore should contain .scholia/index.db by default, got %q", string(data))
	}
}

func TestSaveTransitionNormalizesGivenAndOmitsEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	tx := model.Transition{
		ID:     "T-1",
		Action: "act.x",
		Given:  []string{"cond.b", "cond.a", "cond.b"}, // 未ソート＋重複
		Then:   []string{"eff.two", "eff.one"},         // 順序保存
	}
	if err := s.SaveTransition(tx); err != nil {
		t.Fatalf("SaveTransition: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, ".scholia", "transitions", "T-1.json"))
	if err != nil {
		t.Fatalf("read transition file: %v", err)
	}
	content := string(raw)

	if strings.Contains(content, "null") {
		t.Fatalf("written JSON must never contain null: %s", content)
	}
	if !strings.HasSuffix(content, "\n") {
		t.Fatalf("written JSON must end with a trailing newline")
	}

	loaded, err := s.LoadTransition("T-1")
	if err != nil {
		t.Fatalf("LoadTransition: %v", err)
	}
	wantGiven := []string{"cond.a", "cond.b"}
	if len(loaded.Given) != len(wantGiven) || loaded.Given[0] != wantGiven[0] || loaded.Given[1] != wantGiven[1] {
		t.Fatalf("given not sorted/deduped: got %v", loaded.Given)
	}
	wantThen := []string{"eff.two", "eff.one"}
	if len(loaded.Then) != len(wantThen) || loaded.Then[0] != wantThen[0] || loaded.Then[1] != wantThen[1] {
		t.Fatalf("then order not preserved: got %v", loaded.Then)
	}
	if loaded.Tags != nil {
		t.Fatalf("unset tags should be nil (omitempty), got %v", loaded.Tags)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	v := model.VocabEntry{ID: "act.x", Category: model.CategoryAction, Label: "x"}
	if err := s.SaveVocab(v); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(dir, ".scholia", "vocab"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("leftover tmp file after atomic write: %s", e.Name())
		}
	}
	if len(entries) != 1 || entries[0].Name() != "act.x.json" {
		t.Fatalf("unexpected vocab dir contents: %v", entries)
	}
}

func TestLoadAllSnapshotsEverythingAndFlagsIDMismatch(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.SaveVocab(model.VocabEntry{ID: "cond.a", Category: model.CategoryCondition, Label: "a"}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	if err := s.SaveTag(model.Tag{ID: "subject.x", Name: "x"}); err != nil {
		t.Fatalf("SaveTag: %v", err)
	}
	if err := s.SaveTransition(model.Transition{ID: "T-1", Action: "act.x", Then: []string{"eff.x"}}); err != nil {
		t.Fatalf("SaveTransition: %v", err)
	}
	if err := s.SaveDecision(model.Decision{ID: "01D1", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.x"}, Why: "why", At: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}

	// ファイル名と内部 id が食い違うレコードを直接書き込む（id-unique が拾うべき異常データ）。
	mismatchPath := filepath.Join(dir, ".scholia", "vocab", "cond.other-name.json")
	if err := os.WriteFile(mismatchPath, []byte(`{"id":"cond.a","category":"condition","label":"dup"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write mismatch fixture: %v", err)
	}

	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(snap.Vocab) != 2 || len(snap.Tags) != 1 || len(snap.Transitions) != 1 || len(snap.Decisions) != 1 {
		t.Fatalf("unexpected snapshot sizes: %+v", snap)
	}
	if len(snap.IDMismatches) != 1 || snap.IDMismatches[0].File != "cond.other-name.json" {
		t.Fatalf("expected 1 id mismatch for cond.other-name.json, got %+v", snap.IDMismatches)
	}
}

func TestDiscoverWalksUpward(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init: %v", err)
	}
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	s, err := Discover(deep)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	wantDir, err := filepath.Abs(filepath.Join(root, ".scholia"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Dir != wantDir {
		t.Fatalf("Discover found wrong dir: got %s want %s", s.Dir, wantDir)
	}
}
