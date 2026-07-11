package viewer

import (
	"net/http"
	"testing"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/render"
	"github.com/nkenji09/product-memory/internal/store"
)

func TestGetSpec(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/spec/subject.auth", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	report := decodeJSON[render.SpecReport](t, rec)
	if report.Tag.ID != "subject.auth" {
		t.Fatalf("Tag.ID = %q, want subject.auth", report.Tag.ID)
	}
	if len(report.Entries) != 1 || report.Entries[0].Transition.ID != "T-login" {
		t.Fatalf("Entries = %+v, want [T-login] via ancestor expansion", report.Entries)
	}
}

func TestGetSpec_UnknownTagIsNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/spec/does.not.exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestGetRules_ByTag(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/rules?tag=subject.auth", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[rulesResponse](t, rec)
	if len(out.Decisions) != 1 || out.Decisions[0].ID != "d1" {
		t.Fatalf("Decisions = %+v, want [d1]", out.Decisions)
	}
}

func TestGetRules_ByTx(t *testing.T) {
	h, _ := newTestHandler(t)
	// T-login references tag req.auth-happy, whose ancestor subject.auth carries decision d1
	// (cross-cutting rule, §3.5): the --tx selector must surface it via effective tags.
	rec := doRequest(t, h, http.MethodGet, "/api/rules?tx=T-login", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[rulesResponse](t, rec)
	if len(out.Decisions) != 1 || out.Decisions[0].ID != "d1" {
		t.Fatalf("Decisions = %+v, want [d1]", out.Decisions)
	}
}

func TestGetRules_NoSelectorReturnsAllDecisionsChronologically(t *testing.T) {
	// Deliberately not sharing newTestHandler's fixture: every tag/transition
	// there is already reachable from an existing TestGetRules_ByTag/ByTx
	// assertion, so a second decision targeting any of them (or a new tag/
	// transition, which would show up in the facets/tags/transitions list
	// tests) would need to touch several unrelated tests' exact-count
	// assertions just to exercise this one no-selector code path. A
	// dedicated minimal store keeps this test's blast radius to itself.
	s, err := store.Init(t.TempDir())
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	must(s.SaveTag(model.Tag{ID: "subject.auth", Name: "認証", Kind: "subject"}))
	must(s.SaveDecision(model.Decision{
		ID: "d2", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.auth"},
		Why: "後の決定", At: "2026-02-01T00:00:00Z",
	}))
	must(s.SaveDecision(model.Decision{
		ID: "d1", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.auth"},
		Why: "先の決定", At: "2026-01-01T00:00:00Z",
	}))
	h, err := NewHandler(s)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	rec := doRequest(t, h, http.MethodGet, "/api/rules", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[rulesResponse](t, rec)
	// d2 was saved before d1 but carries the earlier `At`, so a pass here
	// confirms the "no selector" mode sorts chronologically rather than
	// merely echoing file/save order.
	if len(out.Decisions) != 2 || out.Decisions[0].ID != "d1" || out.Decisions[1].ID != "d2" {
		t.Fatalf("Decisions = %+v, want [d1, d2] (chronological by At, independent of save order)", out.Decisions)
	}
}

func TestGetRules_MultipleSelectorsIsBadRequest(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/rules?tag=subject.auth&tx=T-login", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestGetLint(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/lint", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[lintResponse](t, rec)
	// decision-coverage (info) fires because T-login has no decision of its own, only a cross-cutting
	// tag decision; no error-level findings are expected from this fixture.
	if out.ErrorCount != 0 {
		t.Fatalf("ErrorCount = %d, want 0: %+v", out.ErrorCount, out.Findings)
	}
}

func TestGetDiff_NonGitDirIsBadRequest(t *testing.T) {
	// The seeded store lives in a plain t.TempDir(), not a git repo, so
	// diff.Diff necessarily fails; this smoke-tests that the endpoint
	// surfaces the failure as a 400 rather than a panic or 500.
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/diff", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
