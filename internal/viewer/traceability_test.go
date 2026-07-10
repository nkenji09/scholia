package viewer

import (
	"net/http"
	"testing"

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

// newTraceabilityTestHandler seeds a fixture dedicated to traceability
// coverage: a child-tag satisfaction path (T-child tags the leaf
// req.auth-happy, whose ancestor req.auth must show as satisfied via
// ancestor expansion, §3.7), a vocab-tag satisfaction path (T-vocab carries
// no tags of its own but references act.submit, which is tagged
// req.vocab-tagged), and a requirement tag with 0 satisfying transitions
// (req.gap) to exercise the gap flag end to end.
func newTraceabilityTestHandler(t *testing.T) http.Handler {
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
	cfg.TagKinds = []string{"requirement"}
	cfg.FacetKinds = []string{"requirement"}
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
		ID: "act.submit", Category: model.CategoryAction, Label: "送信", Kind: "user",
		Tags: []string{"req.vocab-tagged"},
	}))
	must(s.SaveVocab(model.VocabEntry{ID: "eff.done", Category: model.CategoryEffect, Label: "完了", Kind: "state"}))
	must(s.SaveTag(model.Tag{ID: "req.auth", Name: "認証要件", Kind: "requirement"}))
	must(s.SaveTag(model.Tag{ID: "req.auth-happy", Name: "正常系", Kind: "requirement", ParentIDs: []string{"req.auth"}}))
	must(s.SaveTag(model.Tag{ID: "req.vocab-tagged", Name: "vocab 経由", Kind: "requirement"}))
	must(s.SaveTag(model.Tag{ID: "req.gap", Name: "未充足要件", Kind: "requirement"}))
	must(s.SaveTransition(model.Transition{
		ID: "T-child", Action: "act.submit", Then: []string{"eff.done"}, Tags: []string{"req.auth-happy"},
	}))
	must(s.SaveTransition(model.Transition{
		ID: "T-vocab", Action: "act.submit", Then: []string{"eff.done"},
	}))

	handler, err := NewHandler(s)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return handler
}

func TestGetTraceability_ChildTagSatisfiesAncestorRequirement(t *testing.T) {
	h := newTraceabilityTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/traceability?kind=requirement", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[traceabilityResponse](t, rec)
	entry := findTraceabilityEntry(t, out, "req.auth")
	if entry.Gap {
		t.Fatalf("req.auth.Gap = true, want false (T-child satisfies via child tag req.auth-happy)")
	}
	if len(entry.SatisfiedBy) != 1 || entry.SatisfiedBy[0] != "T-child" {
		t.Fatalf("req.auth.SatisfiedBy = %v, want [T-child]", entry.SatisfiedBy)
	}
}

func TestGetTraceability_VocabTagSatisfiesRequirement(t *testing.T) {
	h := newTraceabilityTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/traceability?kind=requirement", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[traceabilityResponse](t, rec)
	entry := findTraceabilityEntry(t, out, "req.vocab-tagged")
	if entry.Gap {
		t.Fatalf("req.vocab-tagged.Gap = true, want false (satisfied via vocab tag on act.submit)")
	}
	want := map[string]bool{"T-child": true, "T-vocab": true}
	if len(entry.SatisfiedBy) != 2 || !want[entry.SatisfiedBy[0]] || !want[entry.SatisfiedBy[1]] {
		t.Fatalf("req.vocab-tagged.SatisfiedBy = %v, want both T-child and T-vocab", entry.SatisfiedBy)
	}
}

func TestGetTraceability_ZeroSatisfiedIsGap(t *testing.T) {
	h := newTraceabilityTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/traceability?kind=requirement", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[traceabilityResponse](t, rec)
	entry := findTraceabilityEntry(t, out, "req.gap")
	if !entry.Gap {
		t.Fatalf("req.gap.Gap = false, want true (0 satisfying transitions)")
	}
	if len(entry.SatisfiedBy) != 0 {
		t.Fatalf("req.gap.SatisfiedBy = %v, want empty", entry.SatisfiedBy)
	}
}

func TestGetTraceability_KindOmittedUsesAllTraceabilityKinds(t *testing.T) {
	h := newTraceabilityTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/traceability", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[traceabilityResponse](t, rec)
	if len(out.Kinds) != 1 || out.Kinds[0] != "requirement" {
		t.Fatalf("Kinds = %v, want [requirement] (from config.traceabilityKinds)", out.Kinds)
	}
	if len(out.Entries) != 4 {
		t.Fatalf("Entries = %+v, want 4 (all requirement tags)", out.Entries)
	}
}

func TestGetTraceability_UndeclaredKindIsBadRequest(t *testing.T) {
	h := newTraceabilityTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/traceability?kind=not-declared", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func findTraceabilityEntry(t *testing.T, out traceabilityResponse, tagID string) index.TraceabilityEntry {
	t.Helper()
	for _, e := range out.Entries {
		if e.Tag.ID == tagID {
			return e
		}
	}
	t.Fatalf("entry %q not found in %+v", tagID, out.Entries)
	return index.TraceabilityEntry{}
}
