package viewer

import (
	"net/http"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

func TestGetFacets(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/facets", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[facetsResponse](t, rec)
	if len(out.FacetKinds) != 2 {
		t.Fatalf("FacetKinds = %v, want 2 kinds", out.FacetKinds)
	}
	// 統一ツリー（§3.8）: kind 非依存の 1 本のフォレスト。fixture の req.auth-happy
	// は parentIds=[subject.auth]（requirement が subject の子）なので、per-kind の
	// 旧レスポンスでは別々の木だったが、統一ツリーでは subject.auth 配下に cross-kind
	// で入れ子になる（唯一のルートは subject.auth）。
	if len(out.Roots) != 1 || out.Roots[0].Tag.ID != "subject.auth" {
		t.Fatalf("Roots = %+v, want single root subject.auth", out.Roots)
	}
	kids := out.Roots[0].Children
	if len(kids) != 1 || kids[0].Tag.ID != "req.auth-happy" {
		t.Fatalf("subject.auth children = %+v, want [req.auth-happy] (cross-kind nesting)", kids)
	}
	if kids[0].Tag.Kind != "requirement" {
		t.Fatalf("nested child kind = %q, want requirement (kind kept as node attribute)", kids[0].Tag.Kind)
	}
}

func TestGetTags_FilterByKind(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/tags?kind=subject", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	tags := decodeJSON[[]model.Tag](t, rec)
	if len(tags) != 1 || tags[0].ID != "subject.auth" {
		t.Fatalf("tags = %+v, want [subject.auth]", tags)
	}
}

func TestGetTags_UndeclaredKindIsBadRequest(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/tags?kind=not-declared", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestGetVocab_FilterByCategory(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/vocab?category=action", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	vocab := decodeJSON[[]model.VocabEntry](t, rec)
	if len(vocab) != 1 || vocab[0].ID != "act.user.login" {
		t.Fatalf("vocab = %+v, want [act.user.login]", vocab)
	}
}

func TestGetVocab_BySubjectDerivesFromTransitions(t *testing.T) {
	h, _ := newTestHandler(t)
	// subject.auth は T-login の実効タグ（req.auth-happy 経由）。T-login の
	// action(act.user.login)＋then(eff.session.issue) を導出し id 昇順で返す。
	rec := doRequest(t, h, http.MethodGet, "/api/vocab?subject=subject.auth", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	vocab := decodeJSON[[]model.VocabEntry](t, rec)
	if len(vocab) != 2 || vocab[0].ID != "act.user.login" || vocab[1].ID != "eff.session.issue" {
		t.Fatalf("vocab = %+v, want [act.user.login eff.session.issue]", vocab)
	}
}

func TestGetVocab_BySubjectEmptyForUnusedTag(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/vocab?subject=subject.nonexistent", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	vocab := decodeJSON[[]model.VocabEntry](t, rec)
	if len(vocab) != 0 {
		t.Fatalf("vocab = %+v, want empty for unused subject", vocab)
	}
}

func TestGetVocab_InvalidCategoryIsBadRequest(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/vocab?category=nope", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
