package viewer

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/index"
)

func TestGetSearch_MatchesEffectiveTagIDOrName(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/search?q=req.auth-happy", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[index.SearchResult](t, rec)
	assertSearchHitsInclude(t, out, "T-login")
}

func TestGetSearch_MatchesTagName(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/search?q="+"認証", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[index.SearchResult](t, rec)
	assertSearchHitsInclude(t, out, "T-login")
}

func TestGetSearch_MatchesVocabIDOrLabel(t *testing.T) {
	h, _ := newTestHandler(t)

	rec := doRequest(t, h, http.MethodGet, "/api/search?q=eff.session.issue", nil)
	out := decodeJSON[index.SearchResult](t, rec)
	assertSearchHitsInclude(t, out, "T-login")

	rec = doRequest(t, h, http.MethodGet, "/api/search?q="+"セッション発行", nil)
	out = decodeJSON[index.SearchResult](t, rec)
	assertSearchHitsInclude(t, out, "T-login")
}

func TestGetSearch_MatchesTransitionID(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/search?q=T-login", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[index.SearchResult](t, rec)
	assertSearchHitsInclude(t, out, "T-login")
}

func TestGetSearch_MatchesKindName(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/search?q=user", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[index.SearchResult](t, rec)
	assertSearchHitsInclude(t, out, "T-login")
}

func TestGetSearch_NoHitsReturnsEmptyArrayNotNull(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/search?q=nope-nothing-matches", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !jsonContainsEmptyArray(body) {
		t.Fatalf("body = %s, want transitions to serialize as [] not null", body)
	}
	out := decodeJSON[index.SearchResult](t, rec)
	if len(out.Transitions) != 0 {
		t.Fatalf("Transitions = %+v, want empty", out.Transitions)
	}
}

func TestGetSearch_EmptyQueryReturnsEmptyResult(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/search?q=", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[index.SearchResult](t, rec)
	if len(out.Transitions) != 0 {
		t.Fatalf("Transitions = %+v, want empty for blank q", out.Transitions)
	}
}

func assertSearchHitsInclude(t *testing.T, out index.SearchResult, txID string) {
	t.Helper()
	for _, tx := range out.Transitions {
		if tx.ID == txID {
			return
		}
	}
	t.Fatalf("Transitions = %+v, want to include %q", out.Transitions, txID)
}

// jsonContainsEmptyArray is a light substring check (rather than decoding
// twice) that the raw response body encodes transitions as `[]`, not the Go
// zero-value `null` a nil slice would otherwise serialize as.
func jsonContainsEmptyArray(body string) bool {
	return strings.Contains(body, `"transitions": []`) || strings.Contains(body, `"transitions":[]`)
}
