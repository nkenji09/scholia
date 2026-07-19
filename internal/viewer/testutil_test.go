package viewer

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// newTestHandler builds a viewer HTTP handler over a freshly seeded .scholia in
// a t.TempDir(), mirroring the DESIGN §3.2-§3.6 example records so handlers
// can be exercised end to end via httptest.
func newTestHandler(t *testing.T) (http.Handler, *store.Store) {
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
	cfg.TagKinds = []model.KindDecl{{ID: "subject"}, {ID: "requirement"}}
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
	must(s.SaveVocab(model.VocabEntry{ID: "act.user.login", Category: model.CategoryAction, Label: "ログイン", Kind: "user"}))
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

	handler, err := NewHandler(s)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return handler, s
}

func doRequest(t *testing.T, h http.Handler, method, target string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode JSON: %v\nbody: %s", err, rec.Body.String())
	}
	return v
}
