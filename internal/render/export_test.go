package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

// seedExportStore mirrors internal/viewer/testutil_test.go's fixture (same
// record shapes) so export can be exercised against a project that exists
// only on disk (ExportHTML takes a *store.Store, not a Snapshot).
func seedExportStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Init(t.TempDir())
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}

	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cfg.Kinds.Action = []string{"user"}
	cfg.Kinds.Effect = []string{"state"}
	cfg.TagKinds = []string{"subject", "requirement"}
	cfg.FacetKinds = []string{"subject", "requirement"}
	cfg.TraceabilityKinds = []string{"requirement"}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	must(s.SaveVocab(model.VocabEntry{
		ID: "act.user.login", Category: model.CategoryAction, Label: "ログイン", Kind: "user",
		Description: "**ログイン**フォームの送信。",
	}))
	must(s.SaveVocab(model.VocabEntry{ID: "eff.session.issue", Category: model.CategoryEffect, Label: "セッション発行", Kind: "state"}))
	must(s.SaveTag(model.Tag{ID: "subject.auth", Name: "認証", Kind: "subject"}))
	must(s.SaveTag(model.Tag{ID: "req.auth-happy", Name: "正常系ログイン", Kind: "requirement", ParentIDs: []string{"subject.auth"}}))
	must(s.SaveTransition(model.Transition{
		ID: "T-login", Action: "act.user.login", Then: []string{"eff.session.issue"}, Tags: []string{"req.auth-happy"},
	}))
	must(s.SaveDecision(model.Decision{
		ID: "d1", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.auth"},
		Why: "認証は httpOnly cookie で発行", Ref: "PR#1", At: "2026-01-01T00:00:00Z",
	}))

	return s
}

// extractStaticPayload pulls the JSON assigned to window.__PMEM_STATIC__ out
// of an exported index.html so tests can assert on the baked data directly.
func extractStaticPayload(t *testing.T, html string) staticData {
	t.Helper()
	const marker = "window.__PMEM_STATIC__ = "
	start := strings.Index(html, marker)
	if start == -1 {
		t.Fatalf("window.__PMEM_STATIC__ assignment not found in output")
	}
	start += len(marker)
	end := strings.Index(html[start:], ";\n</script>")
	if end == -1 {
		t.Fatalf("could not find end of window.__PMEM_STATIC__ assignment")
	}
	var data staticData
	if err := json.Unmarshal([]byte(html[start:start+end]), &data); err != nil {
		t.Fatalf("unmarshal baked static data: %v\n%s", err, html[start:start+end])
	}
	return data
}

func TestExportHTML_WritesSelfContainedIndexHTML(t *testing.T) {
	s := seedExportStore(t)
	outDir := filepath.Join(t.TempDir(), "site")

	if err := ExportHTML(s, outDir); err != nil {
		t.Fatalf("ExportHTML: %v", err)
	}

	indexPath := filepath.Join(outDir, "index.html")
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read exported index.html: %v", err)
	}
	html := string(raw)

	if !strings.Contains(html, "window.__PMEM_STATIC__ = ") {
		t.Fatal("exported index.html does not inline window.__PMEM_STATIC__")
	}
	if strings.Contains(html, `src="/assets`) || strings.Contains(html, `href="/assets`) {
		t.Fatal("exported index.html still references separate /assets files — not self-contained for file://")
	}
	if !strings.Contains(html, `<style>`) {
		t.Fatal("exported index.html does not inline CSS via <style>")
	}
	// The entry chunk (and, when the SPA imports one, its dynamically-loaded
	// dependents — see export_bundle.go) is inlined as a plain <script> that
	// resolves each chunk's source to a Blob URL at load time, not as a
	// literal <script type="module"> — a real module fetch is blocked by
	// Chrome's file: CORS policy (this file's own doc comment). The
	// resolver's own error string is a stable signature that this bootstrap
	// actually landed, independent of the entry bundle's (hashed, minified)
	// content.
	if strings.Contains(html, `<script type="module">`) {
		t.Fatal("exported index.html inlines the SPA bundle as a literal <script type=\"module\"> — that fetch is blocked under file://")
	}
	if !strings.Contains(html, "pmem export: missing inlined module ") {
		t.Fatal("exported index.html does not inline the SPA bundle via the offline chunk resolver")
	}

	data := extractStaticPayload(t, html)

	if data.Config.TagKinds == nil || len(data.Config.TagKinds) != 2 {
		t.Fatalf("baked config.tagKinds = %v, want 2 entries", data.Config.TagKinds)
	}

	all, ok := data.TransitionsByTag[""]
	if !ok || len(all.Transitions) != 1 || all.Transitions[0].ID != "T-login" {
		t.Fatalf("baked transitionsByTag[\"\"] = %+v, want [T-login]", all)
	}

	byTag, ok := data.TransitionsByTag["subject.auth"]
	if !ok || len(byTag.Transitions) != 1 || byTag.Transitions[0].ID != "T-login" {
		t.Fatalf("baked transitionsByTag[subject.auth] = %+v, want [T-login] via ancestor expansion", byTag)
	}

	detail, ok := data.TransitionDetail["T-login"]
	if !ok {
		t.Fatal("baked transitionDetail missing T-login")
	}
	if detail.ActionLabel != "ログイン" {
		t.Fatalf("detail.ActionLabel = %q, want ログイン", detail.ActionLabel)
	}
	if len(detail.Rules) != 1 || detail.Rules[0].ID != "d1" {
		t.Fatalf("detail.Rules = %+v, want [d1] via effective-tag cross-cutting rule", detail.Rules)
	}

	if len(data.Traceability.Entries) != 1 || data.Traceability.Entries[0].Tag.ID != "req.auth-happy" {
		t.Fatalf("baked traceability entries = %+v, want [req.auth-happy]", data.Traceability.Entries)
	}
	if data.Traceability.Entries[0].Gap {
		t.Fatal("req.auth-happy is satisfied by T-login, want gap=false")
	}

	found := false
	for _, doc := range data.SearchCorpus {
		if doc.TransitionID == "T-login" {
			found = true
		}
	}
	if !found {
		t.Fatal("baked searchCorpus missing T-login")
	}

	if _, ok := data.Spec["subject.auth"]; !ok {
		t.Fatal("baked spec missing subject.auth")
	}

	if len(data.Tags) != 2 {
		t.Fatalf("baked tags = %+v, want 2 entries (subject.auth, req.auth-happy)", data.Tags)
	}
	if len(data.Vocab) != 2 {
		t.Fatalf("baked vocab = %+v, want 2 entries (act.user.login, eff.session.issue)", data.Vocab)
	}
	if data.Vocab[0].ID != "act.user.login" || data.Vocab[0].Description == "" {
		t.Fatalf("baked vocab[0] = %+v, want act.user.login with its markdown description", data.Vocab[0])
	}

	if len(data.Decisions) != 1 || data.Decisions[0].ID != "d1" {
		t.Fatalf("baked decisions = %+v, want [d1] (HOME's recent-decisions widget needs this in static exports too)", data.Decisions)
	}
}

func TestExportHTML_CreatesTargetDir(t *testing.T) {
	s := seedExportStore(t)
	outDir := filepath.Join(t.TempDir(), "nested", "site")

	if err := ExportHTML(s, outDir); err != nil {
		t.Fatalf("ExportHTML: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		t.Fatalf("index.html not created under nested dir: %v", err)
	}
}
