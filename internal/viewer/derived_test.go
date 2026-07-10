package viewer

import (
	"net/http"
	"testing"

	"github.com/nkenji09/product-memory/internal/render"
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
