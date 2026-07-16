package viewer

import (
	"net/http"
	"testing"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
)

// TestDeleteTransition_RemovesFile locks in §8.8 P5's「削除」write: the
// working-tree file disappears (uncommitted — newTestHandler's store has no
// git repo at all, so there is nothing for the handler to even try to
// touch), and nothing else in the fixture is affected.
func TestDeleteTransition_RemovesFile(t *testing.T) {
	h, s := newTestHandler(t)
	if !s.TransitionExists("T-login") {
		t.Fatalf("fixture missing T-login, test assumption broken")
	}

	rec := doRequest(t, h, http.MethodDelete, "/api/transitions/T-login", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	got := decodeJSON[deleteTransitionResponse](t, rec)
	if got.ID != "T-login" {
		t.Fatalf("ID = %q, want T-login", got.ID)
	}
	if s.TransitionExists("T-login") {
		t.Fatalf("T-login still exists after DELETE")
	}

	// The fixture's unrelated tag (subject.auth) and its decision (d1,
	// targeting the tag, not this transition) must survive untouched —
	// the delete must not cascade beyond the one transition file.
	if !s.TagExists("subject.auth") {
		t.Fatalf("unrelated tag subject.auth was removed — delete cascaded beyond the transition")
	}
	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	found := false
	for _, d := range snap.Decisions {
		if d.ID == "d1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unrelated decision d1 was removed — delete cascaded beyond the transition")
	}
}

// TestDeleteTransition_UnknownID404 confirms deleting a nonexistent
// transition 404s without touching anything.
func TestDeleteTransition_UnknownID404(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodDelete, "/api/transitions/T-does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

// TestDeleteTransition_InvalidIDRejected mirrors the create-path
// path-traversal guard for the delete path's {id} URL segment. A bare "."
// or ".." never reaches deleteTransitionHandler at all — net/http's
// ServeMux cleans the path and 307-redirects before routing — so those are
// asserted as "never routed here" rather than 400. A percent-encoded
// separator (`%2F`) *does* reach the handler with the decoded separator
// still in r.PathValue("id"), which is exactly what validTransitionID
// exists to catch (400).
func TestDeleteTransition_InvalidIDRejected(t *testing.T) {
	h, s := newTestHandler(t)
	for _, id := range []string{"..", "."} {
		rec := doRequest(t, h, http.MethodDelete, "/api/transitions/"+id, nil)
		if rec.Code != http.StatusTemporaryRedirect {
			t.Fatalf("id=%q: status = %d, want 307 (mux path-cleaning redirect, never reaches the handler)", id, rec.Code)
		}
	}
	for _, id := range []string{"a%2Fb", "..%2Fescape"} {
		rec := doRequest(t, h, http.MethodDelete, "/api/transitions/"+id, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("id=%q: status = %d, want 400: %s", id, rec.Code, rec.Body.String())
		}
	}
	if !s.TransitionExists("T-login") {
		t.Fatalf("T-login must survive every rejected/redirected delete attempt above")
	}
}

// TestDeleteTransition_ReferencedByDecisionConflict is the「参照整合」guard
// the handoff calls out explicitly: `scholia lint`'s decision-target rule is
// SeverityError (verified via internal/lint), so deleting a transition a
// decision still targets would leave `scholia lint` red. The viewer refuses
// (409) rather than cascading the deletion into the decision file the way
// `scholia tx rm` does — nothing is removed, and the pre-existing decision
// survives.
func TestDeleteTransition_ReferencedByDecisionConflict(t *testing.T) {
	h, s := newTestHandler(t)
	if err := s.SaveDecision(model.Decision{
		ID:     "d-tx",
		Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-login"},
		Why:    "この遷移は現行仕様のまま維持する",
		At:     "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("seed decision targeting T-login: %v", err)
	}

	rec := doRequest(t, h, http.MethodDelete, "/api/transitions/T-login", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if !s.TransitionExists("T-login") {
		t.Fatalf("T-login was removed despite a referencing decision — should have been refused")
	}
	if _, err := s.LoadTransition("T-login"); err != nil {
		t.Fatalf("LoadTransition after refused delete: %v", err)
	}
}

// TestDeleteTransition_LintCleanAfterUnreferencedDelete confirms `scholia
// lint`（internal/lint.Run）stays clean once an unreferenced transition is
// deleted — the handoff's「削除後 lint の挙動」check.
func TestDeleteTransition_LintCleanAfterUnreferencedDelete(t *testing.T) {
	h, s := newTestHandler(t)
	rec := doRequest(t, h, http.MethodDelete, "/api/transitions/T-login", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if findings := lint.Run(snap); lint.HasError(findings) {
		t.Fatalf("lint has errors after unreferenced delete: %+v", findings)
	}
}
