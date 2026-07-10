package viewer

import (
	"net/http"
	"testing"

	"github.com/nkenji09/product-memory/internal/model"
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
	subjectTree := out.Trees["subject"]
	if len(subjectTree) != 1 || subjectTree[0].Tag.ID != "subject.auth" {
		t.Fatalf("Trees[subject] = %+v, want root subject.auth", subjectTree)
	}
	reqTree := out.Trees["requirement"]
	if len(reqTree) != 1 || reqTree[0].Tag.ID != "req.auth-happy" {
		t.Fatalf("Trees[requirement] = %+v, want root req.auth-happy", reqTree)
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

func TestGetVocab_InvalidCategoryIsBadRequest(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/vocab?category=nope", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
